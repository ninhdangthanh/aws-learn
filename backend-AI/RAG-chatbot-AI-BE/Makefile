# Makefile for RAG chatbot backend

.PHONY: run-api run-worker migrate-up migrate-down docker-up docker-down build test tidy

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

build:
	go build ./cmd/api
	go build ./cmd/worker

tidy:
	go mod tidy

migrate-up:
	migrate -path db/migrations -database "$(POSTGRES_DSN)" up

migrate-down:
	migrate -path db/migrations -database "$(POSTGRES_DSN)" down

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

test:
	go test ./...
