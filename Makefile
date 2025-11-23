.PHONY: help build run test clean docker-up docker-down migrate

help: ## Display this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build all services
	@echo "Building all services..."
	cd services/auth && go build -o ../../bin/auth-service ./cmd/main.go
	cd services/project && go build -o ../../bin/project-service ./cmd/main.go
	cd services/websocket && go build -o ../../bin/websocket-service ./cmd/main.go
	cd services/collaboration && go build -o ../../bin/collaboration-service ./cmd/main.go
	cd services/compilation && go build -o ../../bin/compilation-service ./cmd/main.go
	@echo "Build complete!"

run-auth: ## Run auth service
	cd services/auth && go run ./cmd/main.go

run-project: ## Run project service
	cd services/project && go run ./cmd/main.go

run-websocket: ## Run websocket service
	cd services/websocket && go run ./cmd/main.go

run-collaboration: ## Run collaboration service
	cd services/collaboration && go run ./cmd/main.go

run-compilation: ## Run compilation service
	cd services/compilation && go run ./cmd/main.go

test: ## Run tests for all services
	@echo "Running tests..."
	cd services/auth && go test -v ./...
	cd services/project && go test -v ./...
	cd services/websocket && go test -v ./...
	cd services/collaboration && go test -v ./...
	cd services/compilation && go test -v ./...

clean: ## Clean build artifacts
	rm -rf bin/
	find . -name "*.log" -delete

docker-up: ## Start all services with Docker Compose
	docker compose -f deployments/docker/docker-compose.yml up -d

docker-down: ## Stop all services
	docker compose -f deployments/docker/docker-compose.yml down

docker-logs: ## View logs from all services
	docker compose -f deployments/docker/docker-compose.yml logs -f

init-db: ## Initialize database with indexes
	go run scripts/init-db.go

generate-keys: ## Generate JWT RSA keys
	mkdir -p keys
	openssl genrsa -out keys/jwt-private.pem 4096
	openssl rsa -in keys/jwt-private.pem -pubout -out keys/jwt-public.pem
	@echo "JWT keys generated in ./keys/"

frontend-install: ## Install frontend dependencies
	cd frontend && npm install

frontend-dev: ## Run frontend in development mode
	cd frontend && npm run dev

frontend-build: ## Build frontend for production
	cd frontend && npm run build

lint: ## Run linters
	golangci-lint run ./...

deps: ## Download dependencies
	go work sync
	cd services/auth && go mod download
	cd services/project && go mod download
	cd services/websocket && go mod download
	cd services/collaboration && go mod download
	cd services/compilation && go mod download
