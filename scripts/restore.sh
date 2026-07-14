#!/usr/bin/env sh
set -eu

ARCHIVE=${1:-}
DATA_DIR=${DATA_DIR:-/opt/bookshelf/data}

if [ -z "$ARCHIVE" ] || [ ! -f "$ARCHIVE" ]; then
  echo "usage: CONFIRM_RESTORE=yes $0 /path/to/bookshelf-YYYYMMDD-HHMMSS.tar.gz" >&2
  exit 2
fi
if [ "${CONFIRM_RESTORE:-}" != "yes" ]; then
  echo "refusing restore: set CONFIRM_RESTORE=yes after stopping Bookshelf" >&2
  exit 2
fi
case "$DATA_DIR" in ""|"/") echo "unsafe DATA_DIR" >&2; exit 2;; esac
if command -v fuser >/dev/null 2>&1 && [ -f "$DATA_DIR/library.db" ] && fuser "$DATA_DIR/library.db" >/dev/null 2>&1; then
  echo "refusing restore: library.db is still in use; stop the application first" >&2
  exit 1
fi
for command in tar gzip sha256sum sqlite3; do command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }; done

ARCHIVE=$(cd "$(dirname "$ARCHIVE")" && pwd -P)/$(basename "$ARCHIVE")
PARENT=$(cd "$(dirname "$DATA_DIR")" && pwd -P)
BASE=$(basename "$DATA_DIR")
STAMP=$(date -u +%Y%m%d-%H%M%S)
STAGE="$PARENT/.${BASE}.restore-$STAMP-$$"
OLD="$PARENT/.${BASE}.previous-$STAMP-$$"
SAFETY="$PARENT/${BASE}-pre-restore-$STAMP.tar.gz"
LIST="$PARENT/.${BASE}.restore-list-$STAMP-$$"
VERBOSE="$LIST.verbose"
SUCCESS=no
MOVED=no
cleanup() {
  rm -f "$LIST" "$VERBOSE"
  if [ "$SUCCESS" != yes ]; then
    rm -rf "$STAGE"
    if [ "$MOVED" = yes ] && [ ! -e "$DATA_DIR" ] && [ -d "$OLD" ]; then mv "$OLD" "$DATA_DIR"; fi
  fi
}
trap cleanup EXIT INT TERM

if [ -f "$ARCHIVE.sha256" ]; then
  expected=$(awk 'NR==1 {print $1}' "$ARCHIVE.sha256")
  actual=$(sha256sum "$ARCHIVE" | awk '{print $1}')
  [ "$expected" = "$actual" ] || { echo "backup checksum mismatch" >&2; exit 1; }
fi

tar -tzf "$ARCHIVE" > "$LIST"
while IFS= read -r entry; do
  case "$entry" in ""|/*|../*|*/../*|*/..) echo "unsafe archive path: $entry" >&2; exit 1;; esac
done < "$LIST"
tar -tvzf "$ARCHIVE" > "$VERBOSE"
while IFS= read -r line; do
  kind=$(printf '%s' "$line" | cut -c1)
  case "$kind" in l|h) echo "archive links are not allowed" >&2; exit 1;; esac
done < "$VERBOSE"

mkdir -m 0700 "$STAGE"
tar -xzf "$ARCHIVE" -C "$STAGE" --no-same-owner --no-same-permissions
[ -f "$STAGE/library.db" ] && [ -f "$STAGE/backup.json" ] || { echo "backup is missing library.db or backup.json" >&2; exit 1; }
[ "$(sqlite3 "$STAGE/library.db" 'PRAGMA quick_check;')" = "ok" ] || { echo "restored SQLite database failed quick_check" >&2; exit 1; }
for dir in books imports cache trash backups manifests; do mkdir -p "$STAGE/$dir"; done

if [ -d "$DATA_DIR" ]; then
  tar -C "$DATA_DIR" -czf "$SAFETY" .
  mv "$DATA_DIR" "$OLD"
  MOVED=yes
fi
mv "$STAGE" "$DATA_DIR"
SUCCESS=yes
rm -rf "$OLD"
echo "restore complete: $DATA_DIR"
echo "pre-restore safety backup: $SAFETY"
