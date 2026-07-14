#!/usr/bin/env sh
set -eu

DEV_FILE=deploy/docker-compose.dev.yml
PROD_FILE=deploy/docker-compose.prod.yml
MARKER=".bookshelf-volume-check-$$"

dev_compose() {
  docker compose --env-file .env -f "$DEV_FILE" "$@"
}

cleanup() {
  dev_compose down >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker compose --env-file .env -f "$DEV_FILE" config --quiet
docker compose --env-file .env -f "$PROD_FILE" config --quiet

dev_compose up -d --build bookshelf

init_id=$(dev_compose ps -aq storage-init)
[ -n "$init_id" ]
[ "$(docker inspect --format '{{.State.ExitCode}}' "$init_id")" = "0" ]

bookshelf_id=$(dev_compose ps -q bookshelf)
[ -n "$bookshelf_id" ]
i=0
while [ "$i" -lt 30 ]; do
  health=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' "$bookshelf_id")
  [ "$health" = "healthy" ] && break
  [ "$health" != "unhealthy" ] || { echo "bookshelf healthcheck failed" >&2; exit 1; }
  i=$((i + 1))
  sleep 2
done
[ "$health" = "healthy" ] || { echo "bookshelf healthcheck timed out" >&2; exit 1; }

dev_compose exec -T bookshelf sh -c '
  set -eu
  test "$(id -u)" != 0
  test -w /app/data
  for dir in books imports cache trash backups manifests; do test -d "/app/data/$dir"; done
  touch /app/data/.write-test
  rm /app/data/.write-test
  printf persistent > "/app/data/$1"
' sh "$MARKER"

dev_compose down
dev_compose up -d bookshelf
dev_compose exec -T bookshelf sh -c '
  test "$(cat "/app/data/$1")" = persistent
  rm "/app/data/$1"
' sh "$MARKER"

echo "deployment verification passed"
