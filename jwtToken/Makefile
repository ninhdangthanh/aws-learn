.PHONY: up down logs api web install tidy build test psql redis-cli

## Hạ tầng
up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

psql:
	docker exec -it jwt-auth-pg psql -U jwtuser -d jwtauth

redis-cli:
	docker exec -it jwt-auth-redis redis-cli

## Backend
api:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

tidy:
	go mod tidy

test:
	go test ./...

## Frontend
install:
	cd web && npm install

web:
	cd web && npm run dev
