.PHONY: dev test build docker-build docker-up docker-down docker-config docker-verify prod-build prod-up prod-down prod-config backup restore scan integration-test

dev:
	cd backend && go run ./cmd/server

test:
	cd backend && go test ./...
	cd frontend && npm test -- --run

build:
	cd backend && go build ./cmd/server
	cd frontend && npm run build

docker-build:
	docker compose --env-file .env -f deploy/docker-compose.dev.yml build

docker-up:
	docker compose --env-file .env -f deploy/docker-compose.dev.yml up -d --build

docker-down:
	docker compose --env-file .env -f deploy/docker-compose.dev.yml down

docker-config:
	docker compose --env-file .env -f deploy/docker-compose.dev.yml config

docker-verify:
	./scripts/verify-deployment.sh

prod-build:
	docker compose --env-file .env -f deploy/docker-compose.prod.yml build

prod-up:
	docker compose --env-file .env -f deploy/docker-compose.prod.yml up -d --build

prod-down:
	docker compose --env-file .env -f deploy/docker-compose.prod.yml down

prod-config:
	docker compose --env-file .env -f deploy/docker-compose.prod.yml config

backup:
	./scripts/backup.sh

scan:
	./scripts/scan.sh

restore:
	@test -n "$(BACKUP)" || (echo "usage: make restore BACKUP=/path/to/backup.tar.gz CONFIRM_RESTORE=yes" && exit 2)
	CONFIRM_RESTORE=$(CONFIRM_RESTORE) ./scripts/restore.sh "$(BACKUP)"

integration-test:
	./scripts/integration-test.sh
