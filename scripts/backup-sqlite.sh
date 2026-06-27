#!/usr/bin/env bash
set -euo pipefail

DB_PATH="${DB_PATH:-/opt/dailydocs/data/dailydocs.sqlite}"
BACKUP_DIR="${BACKUP_DIR:-/opt/dailydocs/backups}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_PATH="$BACKUP_DIR/dailydocs-$TIMESTAMP.sqlite.gz"

if ! command -v sqlite3 >/dev/null 2>&1; then
	echo "sqlite3 is required." >&2
	exit 1
fi

if [[ ! -r "$DB_PATH" ]]; then
	echo "Database is not readable: $DB_PATH" >&2
	exit 1
fi

mkdir -p "$BACKUP_DIR"

tmp="$(mktemp)"
cleanup() {
	rm -f "$tmp"
}
trap cleanup EXIT

sqlite3 "$DB_PATH" ".backup '$tmp'"
gzip -c "$tmp" >"$BACKUP_PATH"

echo "$BACKUP_PATH"
