#!/usr/bin/env sh
set -eu
: "${BOOKSHELF_URL:=https://localhost}"
: "${ADMIN_USERNAME:?set ADMIN_USERNAME}"
: "${ADMIN_PASSWORD:?set ADMIN_PASSWORD}"
: "${OPDS_USERNAME:?set OPDS_USERNAME}"
: "${OPDS_PASSWORD:?set OPDS_PASSWORD}"
for command in curl jq; do command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }; done
CURL_FLAGS="--fail --silent --show-error"
if [ "${CURL_INSECURE:-}" = "yes" ]; then CURL_FLAGS="$CURL_FLAGS --insecure"; fi
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM
curl $CURL_FLAGS "$BOOKSHELF_URL/api/v1/system/health" | jq -e '.status == "ok"' >/dev/null
jq -n --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{username:$username,password:$password}' > "$WORK/login.json"
curl $CURL_FLAGS -c "$WORK/cookies" -H 'Content-Type: application/json' --data-binary @"$WORK/login.json" "$BOOKSHELF_URL/api/v1/auth/login" >/dev/null
printf '%%PDF-1.7\n/Title (Integration Test) /Author (Bookshelf) /Type /Page >>\n' > "$WORK/test.pdf"
curl $CURL_FLAGS -b "$WORK/cookies" -F "file=@$WORK/test.pdf;type=application/pdf" "$BOOKSHELF_URL/api/v1/imports" > "$WORK/import.json"
BOOK_ID=$(jq -er '.book.id' "$WORK/import.json")
curl $CURL_FLAGS -b "$WORK/cookies" "$BOOKSHELF_URL/api/v1/books/$BOOK_ID" | jq -e '.title == "Integration Test"' >/dev/null
code=$(curl --silent --output /dev/null --write-out '%{http_code}' "$BOOKSHELF_URL/opds")
[ "$code" = "401" ] || { echo "expected unauthenticated OPDS 401, got $code" >&2; exit 1; }
curl $CURL_FLAGS -u "$OPDS_USERNAME:$OPDS_PASSWORD" "$BOOKSHELF_URL/opds/all" | grep -q 'http://opds-spec.org/acquisition'
curl $CURL_FLAGS -b "$WORK/cookies" -X DELETE "$BOOKSHELF_URL/api/v1/books/$BOOK_ID" >/dev/null
echo "integration test passed"
