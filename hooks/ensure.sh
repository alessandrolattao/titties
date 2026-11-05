#!/bin/sh
# Makes sure the titties binary matching this plugin version, and the configured voice, are in the plugin data dir.
# Runs at SessionStart in the background, and again before a notification in case that one came first.
set -eu

: "${CLAUDE_PLUGIN_ROOT:?}" "${CLAUDE_PLUGIN_DATA:?}"

app="$CLAUDE_PLUGIN_DATA/app"
log="$CLAUDE_PLUGIN_DATA/install.log"
lock="$CLAUDE_PLUGIN_DATA/install.lock"
mkdir -p "$CLAUDE_PLUGIN_DATA"

want="v$(sed -n 's/^ *"version": *"\([^"]*\)".*/\1/p' "$CLAUDE_PLUGIN_ROOT/.claude-plugin/plugin.json" | head -1)"
have=$("$app/titties" version 2>/dev/null | awk '{print $2}' || true)
[ "$have" = "$want" ] && exit 0

# One installer at a time; a stale lock older than 10 minutes is ignored.
if ! mkdir "$lock" 2>/dev/null; then
	if [ -n "$(find "$lock" -maxdepth 0 -mmin +10 2>/dev/null)" ]; then
		rm -rf "$lock" && mkdir "$lock"
	else
		exit 0
	fi
fi
trap 'rm -rf "$lock"' EXIT

{
	echo "== $(date) installing titties $want"
	TITTIES_VERSION="$want" TITTIES_APP_DIR="$app" TITTIES_BIN_DIR="$CLAUDE_PLUGIN_DATA/bin" \
		sh "$CLAUDE_PLUGIN_ROOT/install.sh"
	# Make `titties voice` and the other commands reachable from a terminal, unless titties is already installed.
	if ! command -v titties >/dev/null 2>&1; then
		mkdir -p "$HOME/.local/bin"
		ln -sf "$app/titties" "$HOME/.local/bin/titties"
		echo "linked $HOME/.local/bin/titties"
	fi
	# Download the configured voice now, so the first notification speaks right away.
	"$app/titties" say -o "$CLAUDE_PLUGIN_DATA/warmup.wav" "ok"
	rm -f "$CLAUDE_PLUGIN_DATA/warmup.wav"
} >>"$log" 2>&1
