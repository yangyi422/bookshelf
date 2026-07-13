.PHONY: dev test build docker-build docker-up docker-down backup restore scan integration-test

dev:
	cd backend && go run ./cmd/server

test:
	cd backend && go test ./...
	cd frontend && npm test -- --run

build:
	cd backend && go build ./cmd/server
	cd frontend && npm run build

docker-build:
	docker compose -f deploy/docker-compose.yml build

docker-up:
	docker compose -f deploy/docker-compose.yml up -d --build

docker-down:
	docker compose -f deploy/docker-compose.yml down

backup:
	./scripts/backup.sh

scan:
	./scripts/scan.sh

restore:
	@test -n "$(BACKUP)" || (echo "usage: make restore BACKUP=/path/to/backup.tar.gz CONFIRM_RESTORE=yes" && exit 2)
	CONFIRM_RESTORE=$(CONFIRM_RESTORE) ./scripts/restore.sh "$(BACKUP)"

integration-test:
	./scripts/integration-test.sh
