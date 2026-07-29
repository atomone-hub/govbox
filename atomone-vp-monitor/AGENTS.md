# AGENTS.md

## What this is

A standalone Python tool (stdlib only, no dependencies) that reads the reactions
on **one** Discord message, maps each reactor to an on-chain AtomOne validator,
and reports the voting power of *newly-added* reactors plus the running total as
a share of bonded stake and the distance to the 2/3 threshold.

Built to measure how much voting power has signalled a node upgrade (v4.1.0) by
reacting to the announcement in `#validators-announcement`.

It is unrelated to the Go code in the rest of `govbox` — nothing imports it and it
shares no build.

## Commands

All commands run from this directory.

```bash
python3 monitor.py              # report the delta since last run, persist state
python3 monitor.py --full       # also print the whole cumulative table
python3 monitor.py --dry-run    # report without writing state.json
./check_access.sh               # diagnose token / membership / channel / message access
./run.sh                        # cron wrapper: appends log.txt, writes new.txt on change
```

`check_access.sh` is the first thing to run whenever Discord returns an error — it
isolates which of the four layers failed instead of leaving you with a bare HTTP
code.

## Data flow

```
config.json + token
  → GET message               (discover which emoji have reactions)
  → GET reactions/{emoji}     (paginate reactor list, 100/page)
  → match each reactor to a validator moniker   (3 stages, below)
  → GET bonded validators from chain_api        (voting power = bonded tokens)
  → diff against state.json   (only unseen user IDs are "new")
  → print report, save state.json
```

## Hard rules

1. **Never commit or print the bot token.** It lives in `token` (mode 600) and is
   gitignored. After any move or restructure, verify:
   `git check-ignore -v token state.json` (both listed) and
   `git status --short --untracked-files=all | grep token` (prints nothing).
2. **Never guess a validator match.** `UNMATCHED` is a correct, useful outcome. A
   wrong match silently corrupts the voting-power total, which is the entire
   deliverable — and nothing downstream will catch it. Report the candidate and
   ask; do not write to `aliases.json` unprompted.
3. **Only bonded validators count.** The query filters
   `status=BOND_STATUS_BONDED`. Totals drift by a few hundred ATONE between runs
   as delegations move — that is expected, not a bug.
4. **Don't delete `state.json` to fix a matching problem.** Edits to
   `aliases.json` are applied retroactively by the re-resolve loop in `main()`,
   which revisits every stored record that is still unmatched. Deleting state
   makes every existing reactor report as "new" and loses `first_seen`.

## Publishing

Gated behind `config.publish.enabled` (default false). Appends one message per
batch of newly-matched validators to a thread off the tracked message.

**The single most important invariant:** the publish trigger is a *newly-matched
validator*, never a change in the ATONE total. Bonded stake drifts on every run,
so gating on the number posts forever. The implementation compares a per-user
`published` flag in `state.json`, not any total.

- Idempotent by design: mark `published` only after a successful POST. A failed
  post leaves records unmarked and retries next run — never double-announces.
- `post_to_thread` retries once after `PATCH {archived: false}`, because threads
  auto-archive and posting to an archived thread fails.
- Publishing failures must never abort a run — `discord_send` returns
  `(ok, result)` and never raises, unlike `get_json`, which calls `die()`.
- Needs `CREATE_PUBLIC_THREADS` (1<<35) and `SEND_MESSAGES_IN_THREADS` (1<<38) on
  the channel. Plain `SEND_MESSAGES` is deliberately *not* used and **not
  required** — verified empirically: both thread creation and in-thread posting
  succeed with it denied.
- **`EMBED_LINKS` (1<<14) is required to send an embed** and is easy to miss,
  because a content-only post succeeds while an identical post carrying `embeds`
  returns `403 / 50013` — the error names no permission. If a post 403s, check
  this before anything else. There is deliberately no plain-text fallback: posts
  are embed-only and simply retry next run.
- A thread created from a message has `thread_id == message_id`. That is correct,
  not a bug — don't "fix" it.
- Posting immediately after creating a thread can 403 transiently before Discord
  propagates it. The retry-next-run design absorbs this; don't add a tight retry.
- `--dry-run` implies `--no-publish`.
- Unmatched and excluded reactors never trigger a post — a public thread should
  not broadcast "we could not identify you".

## Name matching (three stages, each reached only if the previous fails)

1. **`aliases.json`** — authoritative, keyed by the **lowercase Discord handle**,
   not the display name. A `null` value means "knowingly not a validator" and
   renders as `skipped` instead of being flagged every run.
2. **Fuzzy** — tokenizes display name and handle on `|`, `-`, `()`, `_` etc.,
   drops generic tokens via `STOPWORDS` (`stake`, `node`, `restake`, `dao`, …),
   and requires ≥0.85 confidence **and** a clear winner over the runner-up.
   Ambiguity is reported, never resolved by picking the top score.
3. **Guild nickname** — one `GET /guilds/{guild_id}/members/{user_id}`, but only
   for reactors stages 1–2 failed on. Many operators put their moniker *only* in
   their server nick (`dluigi` → `Luigi | Umbrella`). Cached in state as `nick` /
   `nick_checked` so a permanently unidentifiable user is never re-fetched.

Monikers are `.strip()`ed when indexed — at least one on-chain moniker has a
leading space. Two Discord accounts resolving to the same moniker are
de-duplicated by moniker before summing.

## Discord API specifics

- The reactions endpoint returns `username` and `global_name` only — **not** the
  guild `nick`. That is why stage 3 exists.
- `GET /guilds/{g}/members/{u}` (single member) does **not** require the
  privileged `GUILD_MEMBERS` intent. The member *list* endpoint does. No
  privileged intents are enabled, and none are needed.
- Emoji in a reaction path is `name:id` for custom emoji and the raw character
  for unicode; percent-encode with `safe=''`.
- Paginate reactors with `?after=<last_user_id>`, 100 per page.
- On `429`, honor `retry_after` from the body. `get_json` already does.
- `Missing Access` (50001) is generic and does **not** imply the bot is in the
  guild. For membership, `GET /guilds/{id}` → 404 `Unknown Guild` and
  `/users/@me/guilds` are both reliable; trust them over the Discord UI, which
  can show an app integration entry with no bot member behind it.

## Operational gotchas on the AtomOne Community server

- **AuthGG** (anti-nuke) kicks non-allowlisted bots with audit-log reason
  `Kicked by AuthGG (Unverified Bot)`. An admin fixes it with
  `/whitelist-unverified-bot`. Discord's own **Verified App** program is *not*
  required — that exists to lift the 100-server cap and would need the owner to
  submit government ID.
- Whitelisting does **not** re-add an already-kicked bot. Re-invite it:
  `https://discord.com/oauth2/authorize?client_id=<app_id>&permissions=66560&integration_type=0&scope=bot`
- `#validators-announcement` has channel-level permission overwrites, so
  server-level permissions do not reach it. The bot needs **View Channels** +
  **Read Message History** granted on that channel directly.
- `66560` = View Channels (1024) + Read Message History (65536). Nothing else.
  Keeping the permission set minimal is what makes it an easy allowlist approval.

## Testing without Discord

There is no test suite. To exercise matching and reporting offline, stub the
network layer and redirect state:

```python
import monitor
monitor.fetch_reactions = lambda cfg, tok: [("✅", "✅", [
    {"id": "1", "username": "itrocket", "global_name": "Sam | ITRocket"}, ...])]
monitor.load_token = lambda: "stub"
monitor.STATE = "/tmp/test_state.json"
monitor.main()
```

The regression benchmark is a replay of the 42 reactors originally read from
screenshots: with `aliases.json` emptied, the fuzzy stage resolved **37/42 with
zero false positives**. Any change to `score_pair`, `name_tokens` or `STOPWORDS`
must keep false positives at zero — compare resolved monikers against the
alias-map run rather than just counting how many matched.

## Scheduling — two distinct mechanisms

- **Claude in-session job** (`CronCreate`): in-memory, fires by injecting a
  prompt into the session, dies when the session exits, auto-expires after 7
  days, and is **invisible to `crontab -l`**.
- **`run.sh` + system crontab**: durable, survives restarts, writes `log.txt` and
  `new.txt`. Cron has no useful working directory, so it needs an absolute path.

Do not assume one implies the other. If asked whether "the cron" is updated,
check both.

## Files

| | |
|---|---|
| `monitor.py` | everything: fetch, match, diff, report |
| `config.json` | guild/channel/message IDs, emoji filter, chain API (committable) |
| `aliases.json` | Discord handle → moniker; `null` = not a validator |
| `check_access.sh` | layered access diagnosis |
| `run.sh` | cron wrapper |
| `token` | bot token, mode 600, **gitignored** |
| `state.json` | per-emoji seen reactors, the delta baseline — **gitignored** |
