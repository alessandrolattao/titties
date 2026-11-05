#!/bin/sh
# Notification hook: says out loud that Claude needs you. The hook payload arrives on stdin.
set -eu

: "${CLAUDE_PLUGIN_ROOT:?}" "${CLAUDE_PLUGIN_DATA:?}"

payload=$(cat)

# Silent unless this session was turned on with /titties:on.
id=$(printf '%s' "$payload" | sed -n 's/.*"session_id" *: *"\([^"]*\)".*/\1/p' | head -1)
case "$id" in
"" | *[!A-Za-z0-9-]*) exit 0 ;;
esac
[ -e "$CLAUDE_PLUGIN_DATA/sessions/$id" ] || exit 0

"$CLAUDE_PLUGIN_ROOT/hooks/ensure.sh" || true

bin="$CLAUDE_PLUGIN_DATA/app/titties"
[ -x "$bin" ] || exit 0

. "$CLAUDE_PLUGIN_ROOT/hooks/queue.sh"
wait_turn || exit 0

# The voice is the one picked with `titties voice` (~/.config/titties/config.json).
printf '%s' "$payload" | "$bin" notify >>"$CLAUDE_PLUGIN_DATA/notify.log" 2>&1
