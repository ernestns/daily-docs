#!/usr/bin/env bash
set -euo pipefail

DB_PATH="${DB_PATH:-/opt/dailydocs/data/dailydocs.sqlite}"
BACKUP_FILE="${1:-}"
RESTORE_CONFIRM="${RESTORE_CONFIRM:-}"
APP_NAME="${APP_NAME:-dailydocs}"

if [[ -z "$BACKUP_FILE" ]]; then
	echo "usage: RESTORE_CONFIRM=replace-dailydocs-db $0 path/to/backup.sqlite.gz" >&2
	exit 1
fi

if [[ "$RESTORE_CONFIRM" != "replace-dailydocs-db" ]]; then
	echo "Set RESTORE_CONFIRM=replace-dailydocs-db to restore over $DB_PATH." >&2
	exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
	echo "sqlite3 is required." >&2
	exit 1
fi

if [[ ! -r "$BACKUP_FILE" ]]; then
	echo "Backup is not readable: $BACKUP_FILE" >&2
	exit 1
fi

restore_tmp="$(mktemp)"
current_backup=""
cleanup() {
	rm -f "$restore_tmp"
}
trap cleanup EXIT

case "$BACKUP_FILE" in
	*.gz)
		gzip -dc "$BACKUP_FILE" >"$restore_tmp"
		;;
	*)
		cp "$BACKUP_FILE" "$restore_tmp"
		;;
esac

integrity="$(sqlite3 "$restore_tmp" "PRAGMA integrity_check;")"
if [[ "$integrity" != "ok" ]]; then
	echo "Backup failed integrity check: $integrity" >&2
	exit 1
fi

if systemctl list-unit-files "$APP_NAME.service" >/dev/null 2>&1; then
	systemctl stop "$APP_NAME.service"
fi

if [[ -f "$DB_PATH" ]]; then
	current_backup="$DB_PATH.pre-restore-$(date -u +%Y%m%dT%H%M%SZ)"
	cp "$DB_PATH" "$current_backup"
fi

install -m 0644 "$restore_tmp" "$DB_PATH"

if systemctl list-unit-files "$APP_NAME.service" >/dev/null 2>&1; then
	systemctl start "$APP_NAME.service"
fi

echo "Restored $DB_PATH from $BACKUP_FILE"
if [[ -n "$current_backup" ]]; then
	echo "Previous database copied to $current_backup"
fi
