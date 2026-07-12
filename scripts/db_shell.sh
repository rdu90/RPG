#!/usr/bin/env bash
# Open a sqlite3 shell directly on a save file, resolving the path the same
# way internal/config.SaveDir/SavePath do. Existed because during M1
# development, verifying what a command actually persisted meant either
# trusting the Go test assertions blindly or writing a one-off throwaway
# program; this gets a human straight to the data.
set -euo pipefail

if [ -z "${1:-}" ]; then
	echo "usage: scripts/db_shell.sh <save-name>" >&2
	exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
	echo "sqlite3 CLI not found on PATH" >&2
	exit 1
fi

base="${XDG_DATA_HOME:-$HOME/.local/share}"
path="$base/rpg/$1.db"

if [ ! -f "$path" ]; then
	echo "no save named '$1' at $path" >&2
	exit 1
fi

exec sqlite3 "$path"
