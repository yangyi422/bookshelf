#!/usr/bin/env sh
set -eu
: "${BOOKSHELF_URL:=http://localhost}"
: "${ADMIN_USERNAME:?set ADMIN_USERNAME}"
: "${ADMIN_PASSWORD:?set ADMIN_PASSWORD}"
: "${OPDS_USERNAME:?set OPDS_USERNAME}"
: "${OPDS_PASSWORD:?set OPDS_PASSWORD}"
for command in curl jq cmp; do command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }; done
CURL_FLAGS="--fail --silent --show-error"
if [ "${CURL_INSECURE:-}" = "yes" ]; then CURL_FLAGS="$CURL_FLAGS --insecure"; fi
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM
TEST_ID="$(date -u +%Y%m%d%H%M%S)-$$"
TEST_TITLE="Integration Test $TEST_ID"
curl $CURL_FLAGS "$BOOKSHELF_URL/api/v1/system/health" | jq -e '.status == "ok"' >/dev/null
jq -n --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{username:$username,password:$password}' > "$WORK/login.json"
curl $CURL_FLAGS -c "$WORK/cookies" -H 'Content-Type: application/json' --data-binary @"$WORK/login.json" "$BOOKSHELF_URL/api/v1/auth/login" >/dev/null
printf '%%PDF-1.7\n/Title (%s) /Author (Bookshelf) /Type /Page >>\n' "$TEST_TITLE" > "$WORK/test-$TEST_ID.pdf"
curl $CURL_FLAGS -b "$WORK/cookies" -F "file=@$WORK/test-$TEST_ID.pdf;type=application/pdf" "$BOOKSHELF_URL/api/v1/imports" > "$WORK/import.json"
BOOK_ID=$(jq -er '.book.id' "$WORK/import.json")
curl $CURL_FLAGS -b "$WORK/cookies" "$BOOKSHELF_URL/api/v1/books/$BOOK_ID" > "$WORK/book.json"
jq -e --arg title "$TEST_TITLE" '.title == $title and .files[0].id != null' "$WORK/book.json" >/dev/null
FILE_ID=$(jq -er '.files[0].id' "$WORK/book.json")
curl $CURL_FLAGS -b "$WORK/cookies" --get --data-urlencode "keyword=$TEST_TITLE" "$BOOKSHELF_URL/api/v1/books" | jq -e --arg id "$BOOK_ID" '.items | any(.id == $id)' >/dev/null
curl $CURL_FLAGS -b "$WORK/cookies" "$BOOKSHELF_URL/api/v1/books/$BOOK_ID/files/$FILE_ID/download" > "$WORK/download.pdf"
cmp "$WORK/test-$TEST_ID.pdf" "$WORK/download.pdf"
code=$(curl --silent --output /dev/null --write-out '%{http_code}' "$BOOKSHELF_URL/opds")
[ "$code" = "401" ] || { echo "expected unauthenticated OPDS 401, got $code" >&2; exit 1; }
curl $CURL_FLAGS -u "$OPDS_USERNAME:$OPDS_PASSWORD" "$BOOKSHELF_URL/opds" | grep -q '<title>Bookshelf</title>'
curl $CURL_FLAGS -u "$OPDS_USERNAME:$OPDS_PASSWORD" "$BOOKSHELF_URL/opds/all" | grep -q 'http://opds-spec.org/acquisition'
curl $CURL_FLAGS -u "$OPDS_USERNAME:$OPDS_PASSWORD" --get --data-urlencode "q=$TEST_TITLE" "$BOOKSHELF_URL/opds/search" | grep -q "$TEST_TITLE"
curl $CURL_FLAGS -b "$WORK/cookies" -X DELETE "$BOOKSHELF_URL/api/v1/books/$BOOK_ID" >/dev/null
curl $CURL_FLAGS -b "$WORK/cookies" "$BOOKSHELF_URL/api/v1/books/trash" | jq -e --arg id "$BOOK_ID" 'any(.id == $id)' >/dev/null
curl $CURL_FLAGS -b "$WORK/cookies" -X POST "$BOOKSHELF_URL/api/v1/books/$BOOK_ID/restore" >/dev/null
curl $CURL_FLAGS -b "$WORK/cookies" "$BOOKSHELF_URL/api/v1/books/$BOOK_ID" | jq -e --arg id "$BOOK_ID" '.id == $id' >/dev/null
curl $CURL_FLAGS -b "$WORK/cookies" -X DELETE "$BOOKSHELF_URL/api/v1/books/$BOOK_ID" >/dev/null
echo "integration test passed"
