.PHONY: all build run test clean docker-up docker-down sqlc-generate migrate-up

# Variables
BINARY_NAME=bin/server
MAIN_PATH=./cmd/server

all: test build

build:
	@echo "==> Building server binary..."
	@go build -ldflags="-s -w" -o $(BINARY_NAME) $(MAIN_PATH)

run:
	@echo "==> Running local server..."
	@go run $(MAIN_PATH)

test:
	@echo "==> Running tests..."
	@go test -v ./...

docker-up:
	@echo "==> Starting docker compose services..."
	docker compose up -d

docker-down:
	@echo "==> Stopping docker compose services..."
	docker compose down

docker-build:
	@echo "==> Building docker image..."
	docker build -t kfdesktopbe:latest .

clean:
	@echo "==> Cleaning build artifacts..."
	@rm -rf bin/ coverage/

sqlc-generate:
	@echo "==> Generating sqlc code..."
	sqlc generate
