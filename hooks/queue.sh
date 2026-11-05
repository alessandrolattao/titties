# Sourced by notify.sh and call.sh: one voice speaks at a time, the others wait their turn.
# The lock lives outside the plugin data dir, so sessions of every Claude Code account queue together.
# shellcheck shell=sh

# wait_turn takes the speakers, or returns 1 when the message waited 2 minutes and is stale.
wait_turn() {
	lock="${TMPDIR:-/tmp}/titties-notify-$(id -u).lock"
	waited=0
	until mkdir "$lock" 2>/dev/null; do
		# A lock older than 2 minutes was left by a hook that died: take it over.
		if [ -n "$(find "$lock" -maxdepth 0 -mmin +2 2>/dev/null)" ]; then
			rmdir "$lock" 2>/dev/null || true
			continue
		fi
		waited=$((waited + 1))
		[ "$waited" -gt 1200 ] && return 1
		sleep 0.1
	done
	trap 'rmdir "$lock" 2>/dev/null || true' EXIT
}
