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


# Frontend helpers
.PHONY: run-frontend dev

run-frontend:
	@echo "Starting frontend (will install deps if needed)"
	cd frontend && npm install && npm run dev

# Run API in background and frontend in foreground (simple dev helper)
dev: 
	@echo "Running API in background and frontend in foreground"
	# start api in background
	(go run ./cmd/api) &
	# give API a moment to start
	sleep 1
	cd frontend && npm install && npm run dev
