#!/usr/bin/env sh
set -eu
: "${BOOKSHELF_URL:=http://localhost:8080}"
: "${BOOKSHELF_COOKIE:?set BOOKSHELF_COOKIE to the authenticated bookshelf_session value}"
curl --fail --silent --show-error -X POST -H "Cookie: bookshelf_session=${BOOKSHELF_COOKIE}" "${BOOKSHELF_URL}/api/v1/system/backups"
