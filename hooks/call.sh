#!/bin/sh
# Says a sentence out loud on behalf of a session, for skills that decide themselves when you are needed
# (the release-manager calls you when it needs a decision). Silent unless the session is turned on.
#   call.sh <plugin-root> <plugin-data-dir> <session-id> <<'EOF'
#   <sentence>
#   EOF
set -eu

CLAUDE_PLUGIN_ROOT=${1:?plugin root}
CLAUDE_PLUGIN_DATA=${2:?plugin data dir}
id=${3:-}
# The sentence comes on stdin, so apostrophes and quotes need no escaping.
text=$(cat)
export CLAUDE_PLUGIN_ROOT CLAUDE_PLUGIN_DATA

case "$id" in
"" | *[!A-Za-z0-9-]*)
	echo "TitTieS: no valid session id, nothing said" >&2
	exit 1
	;;
esac
if [ ! -e "$CLAUDE_PLUGIN_DATA/sessions/$id" ]; then
	echo "TitTieS is off for this session: nothing said"
	exit 0
fi
[ -n "$text" ] || {
	echo "TitTieS: nothing to say" >&2
	exit 1
}

"$CLAUDE_PLUGIN_ROOT/hooks/ensure.sh" || true
bin="$CLAUDE_PLUGIN_DATA/app/titties"
[ -x "$bin" ] || {
	echo "TitTieS is not installed yet: nothing said" >&2
	exit 1
}

. "$CLAUDE_PLUGIN_ROOT/hooks/queue.sh"
wait_turn || {
	echo "TitTieS: the speakers were busy for 2 minutes, nothing said" >&2
	exit 1
}

"$bin" say -- "$text" >>"$CLAUDE_PLUGIN_DATA/notify.log" 2>&1
echo "TitTieS said it."
