.PHONY: all build run test clean docker-up docker-down grpc

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
BINARY_NAME=main

all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v

test:
	$(GOTEST) -v ./...

run:
	$(GORUN) main.go

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# Helper for grpc generation from proto
grpc:
	export PATH="$$PATH:$$(go env GOPATH)/bin" && \
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       grpc/order/order.proto \
		   grpc/user/user.proto \
		   grpc/product/product.proto \
