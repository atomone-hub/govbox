#!/usr/bin/env bash
# Verify the bot token and that the bot can actually see the configured message.
# Diagnoses each failure mode separately instead of just printing an HTTP code.
set -uo pipefail
cd "$(dirname "$(readlink -f "$0")")" || exit 1

TOKEN="${DISCORD_BOT_TOKEN:-$(cat token 2>/dev/null)}"
if [ -z "${TOKEN:-}" ]; then
  echo "FAIL: no token. Write it to ./token (chmod 600) or export DISCORD_BOT_TOKEN."
  exit 1
fi

CHAN=$(python3 -c 'import json;print(json.load(open("config.json"))["channel_id"])' 2>/dev/null)
MSG=$(python3 -c 'import json;print(json.load(open("config.json"))["message_id"])' 2>/dev/null)

api() { # api <path> -> "<http_code>\n<body>"
  curl -sS -m 20 -w '\n%{http_code}' \
    -H "Authorization: Bot $TOKEN" \
    -H 'User-Agent: atomone-vp-monitor (check)' \
    "https://discord.com/api/v10$1"
}

split() { body=$(sed '$d' <<<"$1"); code=$(tail -n1 <<<"$1"); }

echo "1. token ................. "
split "$(api /users/@me)"
case "$code" in
  200) echo "   OK — bot is '$(python3 -c 'import json,sys;d=json.load(sys.stdin);print(d["username"])' <<<"$body")'" ;;
  401) echo "   FAIL: 401 — token invalid or was reset. Reset it in the Bot tab and re-save."; exit 1 ;;
  *)   echo "   FAIL: HTTP $code"; echo "$body"; exit 1 ;;
esac

echo "2. guild membership ...... "
split "$(api /users/@me/guilds)"
if [ "$code" = 200 ]; then
  n=$(python3 -c 'import json,sys;print(len(json.load(sys.stdin)))' <<<"$body")
  echo "   in $n server(s):"
  python3 -c 'import json,sys
for g in json.load(sys.stdin): print("        -", g["name"], g["id"])' <<<"$body"
  GUILD=$(python3 -c 'import json;print(json.load(open("config.json")).get("guild_id",""))' 2>/dev/null)
  if [ -n "${GUILD:-}" ]; then
    split "$(api "/guilds/$GUILD")"
    if [ "$code" = 200 ]; then
      echo "   OK — present in the configured guild $GUILD"
    else
      echo "   FAIL: HTTP $code on the configured guild $GUILD."
      echo "         The bot is NOT in the server that owns your message link."
      echo "         Note that a server named similarly (e.g. an 'AtomOne Community'"
      echo "         vs the main AtomOne server) is a DIFFERENT guild. Re-open the"
      echo "         invite URL and pick the server where the message actually lives:"
      echo "         https://discord.com/oauth2/authorize?client_id=1531698831098380388&permissions=66560&integration_type=0&scope=bot"
      exit 1
    fi
  elif [ "$n" = 0 ]; then
    echo "   FAIL: bot is in 0 servers — the invite URL was never authorized."
    exit 1
  fi
else
  echo "   WARN: HTTP $code"
fi

if [ -z "${CHAN:-}" ] || [ "${CHAN:0:1}" = "<" ]; then
  echo "3. channel/message ....... SKIP — config.json not filled in yet."
  exit 0
fi

echo "3. channel visibility .... "
split "$(api "/channels/$CHAN")"
case "$code" in
  200) echo "   OK — #$(python3 -c 'import json,sys;print(json.load(sys.stdin).get("name","?"))' <<<"$body")" ;;
  403) echo "   FAIL: 403 — a channel permission overwrite is denying the bot View Channel."; exit 1 ;;
  404) echo "   FAIL: 404 — wrong channel_id, or the bot cannot see this channel at all."; exit 1 ;;
  *)   echo "   FAIL: HTTP $code"; echo "$body"; exit 1 ;;
esac

echo "4. message + reactions ... "
split "$(api "/channels/$CHAN/messages/$MSG")"
case "$code" in
  200)
    python3 -c 'import json,sys
m=json.load(sys.stdin)
rs=m.get("reactions") or []
print("   OK — message by", (m.get("author") or {}).get("username","?"))
if not rs:
    print("   WARN: this message has no reactions yet.")
else:
    print(f"   {len(rs)} reaction(s):")
    for r in rs:
        e=r["emoji"]
        name=(":" + e["name"] + ":") if e.get("id") else e["name"]
        print("        " + name + "  " + str(r["count"]) + " reactors")' <<<"$body"
    ;;
  403) echo "   FAIL: 403 — bot lacks Read Message History in this channel."; exit 1 ;;
  404) echo "   FAIL: 404 — wrong message_id (or it was deleted)."; exit 1 ;;
  *)   echo "   FAIL: HTTP $code"; echo "$body"; exit 1 ;;
esac

echo
echo "All good — run: python3 monitor.py"
