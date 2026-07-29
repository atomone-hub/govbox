#!/usr/bin/env bash
# Durable entry point for system cron / systemd timers.
# Appends every run to log.txt and, when there are new reactors, also writes a
# one-shot file (new.txt) you can hook a desktop notification onto.
set -uo pipefail
cd "$(dirname "$(readlink -f "$0")")" || exit 1

out=$(python3 monitor.py 2>&1)
rc=$?

{
  echo "----- $(date -Is) (exit $rc)"
  echo "$out"
} >> log.txt

if [ $rc -ne 0 ]; then
  exit $rc
fi

if grep -q NO_NEW_REACTORS <<<"$out"; then
  rm -f new.txt
else
  printf '%s\n' "$out" > new.txt
  # Uncomment for a desktop popup on each new batch of reactors:
  # command -v notify-send >/dev/null && notify-send "AtomOne VP monitor" \
  #   "$(grep -E 'cumulative' <<<"$out" | head -1)"
fi
