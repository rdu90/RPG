#!/usr/bin/env bash
# Run a single SQL statement against a save file via the pure-Go dbquery
# tool, resolving the path the same way internal/config.SaveDir/SavePath do.
# Complements db_shell.sh for machines without the system sqlite3 CLI
# installed (the project depends on modernc.org/sqlite specifically to avoid
# that kind of environment dependency; db-shell alone didn't honor that).
set -euo pipefail

if [ -z "${1:-}" ] || [ -z "${2:-}" ]; then
	echo "usage: scripts/db_query.sh <save-name> <sql>" >&2
	exit 1
fi

base="${XDG_DATA_HOME:-$HOME/.local/share}"
path="$base/rpg/$1.db"

if [ ! -f "$path" ]; then
	echo "no save named '$1' at $path" >&2
	exit 1
fi

go run ./scripts/dbquery "$path" "$2"
