#!/usr/bin/env bash
# Scaffold the next numbered goose migration file. Numbering and the Up/Down
# marker format must match internal/persistence/migrations exactly, so this
# exists to stop that from being retyped (and mistyped) by hand every time a
# milestone adds schema.
set -euo pipefail

if [ -z "${1:-}" ]; then
	echo "usage: scripts/new_migration.sh <description>" >&2
	exit 1
fi

dir="internal/persistence/migrations"
last=$(find "$dir" -maxdepth 1 -name '[0-9][0-9][0-9][0-9]_*.sql' -printf '%f\n' | grep -oE '^[0-9]{4}' | sort -n | tail -1)
next=$(printf "%04d" $((10#${last:-0} + 1)))
name=$(echo "$1" | tr '[:upper:] ' '[:lower:]_')
file="$dir/${next}_${name}.sql"

if [ -e "$file" ]; then
	echo "refusing to overwrite existing $file" >&2
	exit 1
fi

cat >"$file" <<EOF
-- +goose Up


-- +goose Down

EOF

echo "created $file"
