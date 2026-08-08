#!/usr/bin/env python3
"""
Monitor the reactions on one Discord message and report the AtomOne voting power
of each newly-added reactor.

Reactors are mapped to on-chain validator monikers via aliases.json (exact,
authoritative) with a conservative fuzzy fallback for handles not yet in the map.
Voting power is read live from the AtomOne REST API.

State lives in state.json, so each run reports only the delta since the last run.
The first run has an empty state and therefore reports every current reactor.

Optionally announces each new validator in a Discord thread hanging off the
tracked message (config.publish). Announcements are appended, never edited, and
are triggered only by a newly-matched validator — never by the ATONE total moving,
which it does on every run as delegations shift.

Usage:
    ./monitor.py              # report deltas since last run, then persist state
    ./monitor.py --full       # also reprint the full cumulative table
    ./monitor.py --dry-run    # report but do NOT update state.json (implies --no-publish)
    ./monitor.py --no-publish # report and persist, but post nothing to Discord
"""

import difflib
import json
import os
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone

HERE = os.path.dirname(os.path.abspath(__file__))
CONFIG = os.path.join(HERE, "config.json")
ALIASES = os.path.join(HERE, "aliases.json")
STATE = os.path.join(HERE, "state.json")
TOKEN_FILE = os.path.join(HERE, "token")

DISCORD = "https://discord.com/api/v10"
UA = "atomone-vp-monitor (https://github.com/atomone-hub/atomone, 0.1)"

# Tokens too generic to identify a validator on their own.
STOPWORDS = {
    "stake", "staking", "stakes", "node", "nodes", "nodo", "validator",
    "validators", "restake", "hub", "com", "net", "org", "io", "xyz", "team",
    "crypto", "cosmos", "labs", "lab", "capital", "services", "service",
    "network", "networks", "the", "official", "app", "vc", "inc", "dao",
}


def die(msg, code=1):
    print(f"error: {msg}", file=sys.stderr)
    sys.exit(code)


# ---------------------------------------------------------------- config


def load_config():
    if not os.path.exists(CONFIG):
        die(f"missing {CONFIG} — copy config.example.json and fill it in")
    cfg = json.load(open(CONFIG))
    for k in ("channel_id", "message_id"):
        v = str(cfg.get(k, "")).strip()
        if not v or v.startswith("<"):
            die(f"config.json: '{k}' is not set")
    cfg.setdefault("chain_api", "https://atomone-api.allinbits.com")
    cfg.setdefault("emojis", [])  # [] = auto-discover every reaction
    pub = cfg.setdefault("publish", {})
    pub.setdefault("enabled", False)
    pub.setdefault("thread_name", "Upgrade voting power")
    pub.setdefault("title", "Validator voting power")
    pub.setdefault("max_listed", 12)
    return cfg


def load_token():
    tok = os.environ.get("DISCORD_BOT_TOKEN", "").strip()
    if tok:
        return tok
    if os.path.exists(TOKEN_FILE):
        tok = open(TOKEN_FILE).read().strip()
        if tok:
            return tok
    die(
        "no bot token — write it to ./token (chmod 600) "
        "or export DISCORD_BOT_TOKEN"
    )


# ---------------------------------------------------------------- http


def get_json(url, headers=None, tries=5):
    req = urllib.request.Request(url, headers={"User-Agent": UA, **(headers or {})})
    for attempt in range(tries):
        try:
            with urllib.request.urlopen(req, timeout=30) as r:
                return json.loads(r.read().decode())
        except urllib.error.HTTPError as e:
            body = e.read().decode(errors="replace")[:400]
            if e.code == 429:
                try:
                    wait = float(json.loads(body).get("retry_after", 2))
                except Exception:
                    wait = 2.0
                time.sleep(min(wait + 0.3, 30))
                continue
            if e.code in (500, 502, 503, 504) and attempt < tries - 1:
                time.sleep(2 * (attempt + 1))
                continue
            die(f"HTTP {e.code} on {url}\n{body}")
        except urllib.error.URLError as e:
            if attempt < tries - 1:
                time.sleep(2 * (attempt + 1))
                continue
            die(f"network error on {url}: {e}")
    die(f"gave up on {url}")


def discord_get(path, token):
    return get_json(DISCORD + path, {"Authorization": f"Bot {token}"})


def fetch_nick(guild_id, user_id, token):
    """Guild nickname for a user, or None. Never fatal — this is a best-effort
    fallback, and many validators put their moniker only in their server nick
    (e.g. 'Luigi | Umbrella' for handle 'dluigi'). The single-member endpoint
    does not need the privileged GUILD_MEMBERS intent."""
    req = urllib.request.Request(
        f"{DISCORD}/guilds/{guild_id}/members/{user_id}",
        headers={"User-Agent": UA, "Authorization": f"Bot {token}"},
    )
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            return (json.loads(r.read().decode()) or {}).get("nick") or None
    except Exception:
        return None
    finally:
        time.sleep(0.3)


def discord_send(method, path, token, payload=None):
    """-> (ok, parsed_body | error_string). Never raises: publishing must not
    break a run whose report already succeeded."""
    data = json.dumps(payload).encode() if payload is not None else None
    hdrs = {"User-Agent": UA, "Authorization": f"Bot {token}"}
    if data:
        hdrs["Content-Type"] = "application/json"
    for attempt in range(3):
        req = urllib.request.Request(
            DISCORD + path, data=data, method=method, headers=hdrs
        )
        try:
            with urllib.request.urlopen(req, timeout=30) as r:
                body = r.read().decode()
                return True, (json.loads(body) if body.strip() else {})
        except urllib.error.HTTPError as e:
            b = e.read().decode(errors="replace")[:300]
            if e.code == 429:
                try:
                    time.sleep(min(float(json.loads(b).get("retry_after", 2)) + 0.3, 30))
                except Exception:
                    time.sleep(2)
                continue
            return False, f"HTTP {e.code} {b}"
        except Exception as e:  # noqa: BLE001 — best-effort by design
            if attempt < 2:
                time.sleep(2)
                continue
            return False, str(e)
    return False, "gave up after retries"


# ---------------------------------------------------------------- publishing


def ensure_thread(cfg, state, token):
    """Thread hanging off the tracked message, created once. -> (id|None, err).

    A thread keeps the announcement channel itself untouched; its starter message
    is the announcement, so there is no bot-owned message to edit — which is why
    updates are appended rather than edited in place.
    """
    if state.get("thread_id"):
        return state["thread_id"], None
    ok, res = discord_send(
        "POST",
        f"/channels/{cfg['channel_id']}/messages/{cfg['message_id']}/threads",
        token,
        {"name": cfg["publish"]["thread_name"][:100],
         "auto_archive_duration": 10080},
    )
    if not ok:
        return None, res
    state["thread_id"] = res.get("id")
    return state["thread_id"], None


def post_to_thread(thread_id, embed, token):
    """Append a message. Unarchives first if needed — threads auto-archive."""
    payload = {"embeds": [embed], "allowed_mentions": {"parse": []}}
    ok, res = discord_send("POST", f"/channels/{thread_id}/messages", token, payload)
    if ok:
        return True, res
    discord_send("PATCH", f"/channels/{thread_id}", token, {"archived": False})
    return discord_send("POST", f"/channels/{thread_id}/messages", token, payload)


def bar(frac, width=18):
    n = max(0, min(width, round(frac * width)))
    return "▰" * n + "▱" * (width - n)


def build_embed(cfg, label, arrivals, cum, total_bonded, n_vals, vals, seen, now):
    """arrivals: [(tokens, moniker), ...] already sorted descending."""
    pct = 100 * cum / total_bonded
    need = 2 * total_bonded / 3 - cum
    cap = cfg["publish"]["max_listed"]

    lines = [f"**{m}** — {t / 1e6:,.0f} ATONE ({100 * t / total_bonded:.2f}%)"
             for t, m in arrivals[:cap]]
    if len(arrivals) > cap:
        rest = sum(t for t, _ in arrivals[cap:])
        lines.append(f"…and {len(arrivals) - cap} more totalling "
                     f"{rest / 1e6:,.0f} ATONE")

    fields = [
        {"name": f"Signalled with {label}",
         "value": "\n".join(lines)[:1024], "inline": False},
        {"name": "Total",
         "value": f"**{cum / 1e6:,.0f} ATONE — {pct:.2f}%**\n{n_vals} validators",
         "inline": True},
        {"name": "Still needed for 2/3" if need > 0 else "2/3 reached",
         "value": (f"{need / 1e6:,.0f} ATONE ({100 * need / total_bonded:.2f}%)"
                   if need > 0 else f"+{-need / 1e6:,.0f} ATONE over"),
         "inline": True},
    ]
    missing = sorted(((t, m) for m, t in vals.items() if m not in seen),
                     reverse=True)[:5]
    if missing:
        fields.append({
            "name": "Largest not yet signalled",
            "value": "\n".join(f"{m} — {100 * t / total_bonded:.2f}%"
                               for t, m in missing)[:1024],
            "inline": False,
        })
    return {
        "title": cfg["publish"]["title"],
        "description": f"`{bar(cum / (2 * total_bonded / 3))}`  "
                       f"{pct:.2f}% of {100 * 2 / 3:.2f}% needed",
        "color": 0x2ECC71 if need <= 0 else 0x3498DB,
        "fields": fields,
        "footer": {"text": "bonded stake, read live from chain"},
        "timestamp": now,
    }


# ---------------------------------------------------------------- discord


def emoji_key(emoji):
    """Reaction path segment: 'name:id' for custom emoji, the char for unicode."""
    if emoji.get("id"):
        return f"{emoji['name']}:{emoji['id']}"
    return emoji["name"]


def emoji_label(emoji):
    return f":{emoji['name']}:" if emoji.get("id") else emoji["name"]


def fetch_reactions(cfg, token):
    """-> [(label, key, [user, ...]), ...] for every reaction on the message."""
    msg = discord_get(
        f"/channels/{cfg['channel_id']}/messages/{cfg['message_id']}", token
    )
    reactions = msg.get("reactions") or []
    if not reactions:
        return []

    wanted = {e.strip() for e in cfg.get("emojis", []) if e.strip()}
    out = []
    for r in reactions:
        emoji = r["emoji"]
        key, label = emoji_key(emoji), emoji_label(emoji)
        if wanted and label not in wanted and emoji["name"] not in wanted:
            continue

        users, after = [], None
        while True:
            q = "limit=100" + (f"&after={after}" if after else "")
            page = discord_get(
                f"/channels/{cfg['channel_id']}/messages/{cfg['message_id']}"
                f"/reactions/{urllib.parse.quote(key, safe='')}?{q}",
                token,
            )
            if not page:
                break
            users.extend(page)
            if len(page) < 100:
                break
            after = page[-1]["id"]
            time.sleep(0.3)  # stay well under the rate limit
        out.append((label, key, users))
        time.sleep(0.3)
    return out


# ---------------------------------------------------------------- chain


def fetch_validators(api):
    url = (
        f"{api.rstrip('/')}/cosmos/staking/v1beta1/validators"
        "?status=BOND_STATUS_BONDED&pagination.limit=500"
    )
    vals = get_json(url)["validators"]
    if not vals:
        die("chain API returned no bonded validators")
    # moniker -> tokens; keep the raw moniker for display
    return {v["description"]["moniker"].strip(): int(v["tokens"]) for v in vals}


# ---------------------------------------------------------------- matching


def norm(s):
    return re.sub(r"[^a-z0-9]", "", s.lower())


def name_tokens(s):
    """Split a display name / moniker into candidate identity tokens."""
    out = []
    for part in re.split(r"[|/\\,()\[\]{}•·—–\-_+]+", s or ""):
        n = norm(part)
        if n and n not in STOPWORDS:
            out.append(n)
    whole = norm(s or "")
    if whole and whole not in out and whole not in STOPWORDS:
        out.append(whole)
    return out


def score_pair(a, b):
    """0..1 confidence that token `a` identifies token `b`."""
    if len(a) < 3 or len(b) < 3:
        return 0.0
    if a == b:
        return 1.0
    if len(a) >= 5 and len(b) >= 5 and (a in b or b in a):
        return 0.92
    if len(a) >= 5 and len(b) >= 5:
        r = difflib.SequenceMatcher(None, a, b).ratio()
        if r >= 0.85:
            return r * 0.9
    return 0.0


def match_validator(user, aliases, vals, nick=None):
    """-> (moniker|None, how, note). how in exact-alias/fuzzy/none/excluded.

    `nick` is the guild nickname, when known — often the only place the moniker
    appears, so it is folded into the token sources alongside display and handle.
    """
    handle = (user.get("username") or "").lower()
    display = user.get("global_name") or user.get("username") or ""

    if handle in aliases:
        mon = aliases[handle]
        if mon is None:
            return None, "excluded", "aliases.json: not a validator"
        if mon.strip() in vals:
            return mon.strip(), "exact-alias", ""
        return None, "none", f"alias '{mon}' is not a bonded validator"

    src = name_tokens(display) + name_tokens(handle) + name_tokens(nick or "")
    ranked = []
    for mon in vals:
        best = max(
            (score_pair(t, m) for t in src for m in name_tokens(mon)), default=0.0
        )
        if best > 0:
            ranked.append((best, mon))
    if not ranked:
        return None, "none", "no candidate"
    ranked.sort(reverse=True)
    top, mon = ranked[0]
    if top < 0.85:
        return None, "none", f"best guess '{mon}' too weak ({top:.2f})"
    if len(ranked) > 1 and ranked[1][0] >= top - 0.02:
        others = ", ".join(m for _, m in ranked[:3])
        return None, "none", f"ambiguous between: {others}"
    return mon, "fuzzy", f"confidence {top:.2f}"


# ---------------------------------------------------------------- state


def load_state():
    if os.path.exists(STATE):
        return json.load(open(STATE))
    return {"emojis": {}}


def save_state(state):
    tmp = STATE + ".tmp"
    with open(tmp, "w") as f:
        json.dump(state, f, indent=1, ensure_ascii=False, sort_keys=True)
    os.replace(tmp, STATE)


# ---------------------------------------------------------------- report


def atone(u):
    return f"{u / 1e6:,.0f}"


def main():
    full = "--full" in sys.argv
    dry = "--dry-run" in sys.argv
    no_publish = "--no-publish" in sys.argv or dry

    cfg = load_config()
    token = load_token()
    aliases = {k.lower(): v for k, v in json.load(open(ALIASES)).items()
               if not k.startswith("_")}

    vals = fetch_validators(cfg["chain_api"])
    total_bonded = sum(vals.values())
    reactions = fetch_reactions(cfg, token)
    state = load_state()
    now = datetime.now(timezone.utc).replace(microsecond=0).isoformat()

    print("=" * 72)
    print("AtomOne — Discord reaction voting-power monitor")
    print(f"message  : {cfg['channel_id']}/{cfg['message_id']}")
    print(f"checked  : {now}")
    print(f"bonded   : {atone(total_bonded)} ATONE across {len(vals)} validators")
    print("=" * 72)

    if not reactions:
        print("\nno reactions on this message (or none matched config.emojis)")
        return

    any_new = False
    for label, key, users in reactions:
        slot = state["emojis"].setdefault(key, {"label": label, "users": {}})
        slot["label"] = label
        known = slot["users"]
        new = [u for u in users if u["id"] not in known]

        guild = cfg.get("guild_id")
        for u in new:
            mon, how, note = match_validator(u, aliases, vals)
            nick, checked = None, False
            # only pay for the extra request when display+handle got us nowhere
            if mon is None and how == "none" and guild:
                nick, checked = fetch_nick(guild, u["id"], token), True
                if nick:
                    mon, how, note = match_validator(u, aliases, vals, nick=nick)
                    if mon:
                        note = f"nickname {nick!r}, {note}"
            known[u["id"]] = {
                "username": u.get("username"),
                "global_name": u.get("global_name"),
                "nick": nick,
                "nick_checked": checked,
                "moniker": mon,
                "match": how,
                "note": note,
                "first_seen": now,
            }

        # retry the still-unmatched: aliases.json may have gained an entry, and
        # older records may predate nickname lookup entirely
        for uid, rec in known.items():
            if rec.get("moniker") or rec.get("match") == "excluded":
                continue
            mon, how, note = match_validator(rec, aliases, vals, nick=rec.get("nick"))
            if not mon and guild and not rec.get("nick_checked"):
                rec["nick"] = fetch_nick(guild, uid, token)
                rec["nick_checked"] = True
                if rec["nick"]:
                    mon, how, note = match_validator(
                        rec, aliases, vals, nick=rec["nick"]
                    )
                    if mon:
                        note = f"nickname {rec['nick']!r}, {note}"
            # write back even when still unmatched, so the stored reason stays
            # current after an aliases.json edit (mon is None here anyway)
            rec.update(moniker=mon, match=how, note=note)

        matched = {
            uid: r for uid, r in known.items()
            if r.get("moniker") and r["moniker"] in vals
        }
        # de-duplicate: two Discord accounts could point at one validator
        by_moniker = {}
        for uid, r in matched.items():
            by_moniker.setdefault(r["moniker"], uid)
        cum = sum(vals[m] for m in by_moniker)

        print(f"\n{label}  {len(users)} reactors total, {len(new)} new since last run")
        print("-" * 72)

        if new:
            any_new = True
            rows = []
            for u in new:
                r = known[u["id"]]
                vp = vals.get(r["moniker"] or "", 0)
                rows.append((vp, r))
            rows.sort(key=lambda x: -x[0])
            for vp, r in rows:
                who = r.get("nick") or r["global_name"] or r["username"]
                if vp:
                    print(
                        f"  + {atone(vp):>11} ATONE {100 * vp / total_bonded:6.3f}%  "
                        f"{r['moniker']}"
                    )
                    print(f"                                  via {who} ({r['username']})"
                          + (f" [{r['note']}]" if r["match"] == "fuzzy" else ""))
                elif r["match"] == "excluded":
                    print(f"  + {'—':>11}             skipped  "
                          f"{who} ({r['username']}) — {r['note']}")
                else:
                    print(f"  + {'?':>11}          UNMATCHED  "
                          f"{who} ({r['username']}) — {r['note']}")
        else:
            print("  (no new reactors)")

        pct = 100 * cum / total_bonded
        need = (2 * total_bonded / 3) - cum
        print(f"\n  cumulative: {len(known)} reactors, {len(by_moniker)} validators "
              f"matched, {atone(cum)} ATONE = {pct:.2f}% of bonded stake")
        if need > 0:
            print(f"              {atone(need)} ATONE ({100 * need / total_bonded:.2f}%) "
                  f"short of the 2/3 threshold")
        else:
            print(f"              2/3 threshold reached (+{atone(-need)} ATONE)")

        # Publish only validators not yet announced. The trigger is a new MATCHED
        # reactor, never a change in the ATONE total — bonded stake drifts on
        # every tick as delegations move, which would otherwise post forever.
        if cfg["publish"]["enabled"] and not no_publish:
            arrivals = sorted(
                ((vals[r["moniker"]], r["moniker"]) for r in known.values()
                 if r.get("moniker") in vals and not r.get("published")),
                reverse=True,
            )
            if not arrivals:
                print("  publish: nothing new to announce")
            else:
                tid, err = ensure_thread(cfg, state, token)
                if not tid:
                    print(f"  publish: no thread — {err}")
                else:
                    ok, res = post_to_thread(
                        tid,
                        build_embed(cfg, label, arrivals, cum, total_bonded,
                                    len(by_moniker), vals, set(by_moniker), now),
                        token,
                    )
                    if ok:
                        for r in known.values():
                            if r.get("moniker") in vals:
                                r["published"] = True
                        print(f"  publish: announced {len(arrivals)} validator(s) "
                              f"in thread {tid}")
                    else:
                        print(f"  publish: failed, will retry next run — {res}")

        unmatched = [r for r in known.values()
                     if not r.get("moniker") and r.get("match") != "excluded"]
        if unmatched:
            print(f"  unmatched ({len(unmatched)}) — add to aliases.json to count them:")
            for r in sorted(unmatched, key=lambda x: x["username"] or ""):
                seen_as = r.get("nick") or r.get("global_name") or ""
                print(f"      {r['username']:<32} {seen_as}  — {r['note']}")

        if full and by_moniker:
            print(f"\n  full matched table for {label}:")
            for mon, uid in sorted(by_moniker.items(), key=lambda kv: -vals[kv[0]]):
                vp = vals[mon]
                print(f"      {atone(vp):>11} {100 * vp / total_bonded:6.3f}%  {mon}"
                      f"   ({known[uid]['username']})")

    if dry:
        print("\n[--dry-run] state.json not updated")
    else:
        state["last_run"] = now
        state["message"] = {"channel_id": cfg["channel_id"],
                            "message_id": cfg["message_id"]}
        save_state(state)

    if not any_new:
        print("\nNO_NEW_REACTORS")


if __name__ == "__main__":
    main()
