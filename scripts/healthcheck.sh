#!/usr/bin/env sh
set -eu
: "${BOOKSHELF_URL:=http://localhost:8080}"
curl --fail --silent --show-error "${BOOKSHELF_URL}/api/v1/system/health"
