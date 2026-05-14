.PHONY: build run dev test clean docker-build docker-up docker-down

build:
	go build -o backend .

run:
	./backend

dev:
	DB_HOST=localhost DB_PORT=5432 DB_USER=kafka_user DB_PASSWORD=kafka_pass DB_NAME=commerce_db \
	REDIS_HOST=localhost REDIS_PORT=6379 \
	KAFKA_BROKER=localhost:9092 \
	SERVER_PORT=8000 \
	go run main.go

test:
	go test ./...

clean:
	rm -f backend

docker-build:
	docker build -t kafka-commerce-backend .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

deps:
	go mod download
	go mod tidy

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...
