.PHONY: run-api run-worker build test tidy migrate-up migrate-down docker-up docker-down

ifneq (,$(wildcard ./.env))
include .env
export
endif

GO ?= go
COMPOSE ?= docker compose
MIGRATE ?= $(GO) run -tags=postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0

run-api:
	$(GO) run ./cmd/api

run-worker:
	$(GO) run ./cmd/worker

build:
	$(GO) build ./cmd/api
	$(GO) build ./cmd/worker

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

migrate-up:
	$(MIGRATE) -path db/migrations -database "$(POSTGRES_DSN)" up

migrate-down:
	$(MIGRATE) -path db/migrations -database "$(POSTGRES_DSN)" down

docker-up:
	$(COMPOSE) up -d

docker-down:
	$(COMPOSE) down
