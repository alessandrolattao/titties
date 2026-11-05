#!/bin/sh
# Turns voice notifications on or off for one session. Nothing speaks unless its session was turned on.
#   session.sh on|off <session-id> <plugin-data-dir>   from the /titties:on and /titties:off skills
#   session.sh end                                      as the SessionEnd hook, payload on stdin
set -eu

action=${1:?on, off or end}
if [ "$action" = end ]; then
	: "${CLAUDE_PLUGIN_DATA:?}"
	data=$CLAUDE_PLUGIN_DATA
	id=$(sed -n 's/.*"session_id" *: *"\([^"]*\)".*/\1/p' | head -1)
else
	id=${2:-}
	data=${3:?plugin data dir}
fi

# A session id is a UUID: refuse anything else, it ends up in a path.
case "$id" in
"" | *[!A-Za-z0-9-]*)
	echo "TitTieS: no valid session id, nothing changed" >&2
	exit 1
	;;
esac

dir="$data/sessions"
case "$action" in
on)
	mkdir -p "$dir"
	: >"$dir/$id"
	echo "TitTieS is on for this session: it will call you out loud when it needs you."
	;;
off)
	rm -f "$dir/$id"
	echo "TitTieS is off for this session."
	;;
end)
	rm -f "$dir/$id"
	;;
*)
	echo "usage: session.sh on|off <session-id> <plugin-data-dir> | end" >&2
	exit 2
	;;
esac
