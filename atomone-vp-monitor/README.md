# AtomOne Discord reaction → voting-power monitor

Watches the reactions on one Discord message and reports the AtomOne voting power
of each **newly-added** reactor, plus the running cumulative total as a share of
bonded stake and the distance to the 2/3 threshold.

Built to track which validators have signalled a version upgrade (e.g. v4.1.0) by
reacting to an announcement message.

## Setup

### 1. Discord bot

The bot only needs to *read*, so keep its permissions minimal.

1. https://discord.com/developers/applications → **New Application**
2. **Bot** tab → **Reset Token** → copy it.
   No privileged gateway intents are required.
3. **OAuth2 → URL Generator**: scope `bot`, permissions
   **View Channels** + **Read Message History**. Open the generated URL and add
   the bot to the AtomOne server. (A server admin must do this step.)
4. Store the token — never inline it in a cron line or commit it:

   ```bash
   printf '%s\n' 'YOUR_BOT_TOKEN' > token
   chmod 600 token
   ```

   `DISCORD_BOT_TOKEN` in the environment takes precedence if set.

All commands below are run from this directory.

### 2. Point it at the message

Enable Discord **Settings → Advanced → Developer Mode**, right-click the message
→ **Copy Message Link**. The link looks like:

```
https://discord.com/channels/<guild_id>/<channel_id>/<message_id>
```

Put the last two into `config.json`:

```json
{
  "channel_id": "1234567890",
  "message_id": "9876543210",
  "emojis": [],
  "chain_api": "https://atomone-api.allinbits.com"
}
```

`"emojis": []` tracks **every** reaction on the message and reports each one
separately. Restrict it if you only care about specific ones, e.g. `["✅"]`, or
`[":atomone:"]` for a custom emoji.

The live config is set to `["✅"]` — the 🔥 reaction on the v4.1.0 announcement is
ignored (its 5 reactors were all a subset of the ✅ set anyway). To start tracking
another emoji later, add it here; its reactors will all count as "new" on the
first run, since there's no prior baseline for it in `state.json`.

### 3. Run it

```bash
python3 monitor.py
```

Flags: `--full` also prints the whole cumulative table; `--dry-run` reports
without writing `state.json`.

The **first run reports every current reactor** — that establishes the baseline in
`state.json`. Every run after that reports only the delta.

## Scheduling

A Claude in-session cron job is already checking every 30 minutes (`:13` and
`:43`), but it is **in-memory only** — it dies when the Claude session exits and
auto-expires after 7 days.

For something durable, install the system-cron line:

Cron needs an absolute path, so let the shell fill it in from here:

```bash
( crontab -l 2>/dev/null; echo "13,43 * * * * $PWD/run.sh" ) | crontab -
```

`run.sh` appends each run to `log.txt`, and writes `new.txt` only when there were
new reactors (handy for a `notify-send` hook — see the commented line inside).

## Publishing to Discord

Off by default. When `publish.enabled` is true, each newly-matched validator is
**appended** to a thread hanging off the tracked message:

```json
"publish": {
  "enabled": true,
  "thread_name": "v4.1.0 upgrade — voting power",
  "title": "v4.1.0 upgrade — validator voting power",
  "max_listed": 12
}
```

The bot needs two extra permissions **on that channel**: `Créer des fils publics`
(Create Public Threads) and `Envoyer des messages dans les fils` (Send Messages in
Threads). Plain *Send Messages* is **not** needed — verified: thread creation and
in-thread posting both work without it, and nothing is ever posted to the channel
itself, which keeps the announcement channel untouched.

Posts are embeds, which additionally requires `Intégrer des liens`
(`EMBED_LINKS`). Without it Discord returns `403 / 50013` and the post is retried
on the next run — the reading itself is unaffected. Note the failure mode is
confusing: a content-only message succeeds while the same message carrying
`embeds` is refused, and the error names no permission.

Full invite integer for read + threads + embeds is `309237728256`:

| Permission | Bit | Value |
|---|---|---|
| View Channels | 1<<10 | 1024 |
| Embed Links | 1<<14 | 16384 |
| Read Message History | 1<<16 | 65536 |
| Create Public Threads | 1<<35 | 34359738368 |
| Send Messages in Threads | 1<<38 | 274877906944 |

Behaviour worth knowing:

- **The trigger is a newly-matched validator, never a change in the total.**
  Bonded stake drifts every single run as delegations move, so gating on the
  number would post forever.
- Posts are **appended, not edited**. A thread created from a message has that
  message as its starter, so there is no bot-owned message to update — and the
  append log doubles as a timeline of when stake arrived.
- Each post is **self-contained**: new arrivals, running total, gap to 2/3, and
  the five largest validators that still haven't signalled.
- Announced validators are marked `published` in `state.json`. If a post fails,
  they stay unmarked and are retried on the next run, so nothing is lost and
  nothing is announced twice.
- New reactors that are *unmatched* or *excluded* never post — they only show up
  in the terminal report.
- `--dry-run` implies `--no-publish`. Use `--no-publish` alone to update state
  without posting.
- The **first** post announces every already-matched validator as a baseline
  (listing the top `max_listed` and summarising the rest). To start the thread
  from "future arrivals only", set `"published": true` on the existing records in
  `state.json` first.

## Name matching

Discord handles are mapped to on-chain monikers in three stages, each only
reached when the previous one fails:

1. **`aliases.json`** — authoritative, keyed by the lowercase Discord *handle*
   (not the display name). Seeded with the 39 validators identified from the
   original screenshots. `null` marks a reactor knowingly not tied to a
   validator, so they are listed as `skipped` rather than nagged about.
2. **Fuzzy fallback** — for handles not in the map. Tokenizes the display name
   and handle (splitting on `|`, `-`, `()`, …), drops generic words like
   `stake`/`node`/`restake`, and requires ≥0.85 confidence *and* a clear winner
   over the runner-up. On a replay of the 42 original reactors it resolved 37
   with **zero false positives**; the 5 it declined were flagged `UNMATCHED`
   rather than guessed.
3. **Guild nickname lookup** — only when 1 and 2 both fail. The reactions
   endpoint returns just `username` and `global_name`, but many operators put
   their moniker solely in their *server nickname*: handle `dluigi` is
   `Luigi | Umbrella` in the guild, and `buktapabu4` is `BukTaPaBu4 | 79anvi`.
   One `GET /guilds/{guild_id}/members/{user_id}` per otherwise-unmatched
   reactor recovers those; it needs `guild_id` in `config.json` and works
   without the privileged GUILD_MEMBERS intent. Results are cached in
   `state.json` (`nick`, `nick_checked`) so a permanently unmatched user is
   never re-fetched.

Anything reported as `UNMATCHED` is **not counted** in the total. Add it to
`aliases.json` and the next run picks it up retroactively — no need to reset
state.

## Caveats

- Only **bonded** validators count; VP is live bonded tokens, so the total drifts
  slightly between runs as delegations move.
- Two Discord accounts pointing at the same moniker are de-duplicated.
- Reacting is self-reported: it means a validator *said* they upgraded, not that
  their node is actually running the new binary.

## Files

| | |
|---|---|
| `monitor.py` | the script |
| `config.json` | guild/channel/message IDs, emoji filter, chain API |
| `aliases.json` | Discord handle → validator moniker |
| `check_access.sh` | diagnoses token / membership / channel / message access |
| `token` | bot token, `chmod 600`, **gitignored** |
| `state.json` | seen reactors per emoji (the delta baseline), gitignored |
| `run.sh` | cron wrapper: appends `log.txt`, writes `new.txt` |

## Note on living inside the govbox repo

This directory sits in a git working tree. The local `.gitignore` keeps `token`,
`state.json`, `log.txt`, `new.txt` and `__pycache__/` out of git — verify with:

```bash
git check-ignore -v token state.json     # both must be listed
git status --short --untracked-files=all | grep token   # must print nothing
```

Note that `config.json`
**is** committable and contains the guild/channel/message IDs — they are not
secrets, but they are AtomOne-specific, so move them to `config.example.json`
placeholders if this is ever published more widely.
