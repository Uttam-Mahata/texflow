.PHONY: help build run test clean docker-up docker-down migrate k8s-deploy k8s-delete k8s-status k8s-logs docker-build

help: ## Display this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

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

copy-keys: ## Copy keys to service directories
	@if [ -d "keys" ]; then \
		echo "Copying keys to services..."; \
		cp -r keys services/auth/; \
		cp -r keys services/project/; \
		cp -r keys services/websocket/; \
		cp -r keys services/collaboration/; \
		cp -r keys services/compilation/; \
	else \
		echo "Keys directory not found. Skipping key copy."; \
	fi

docker-up: copy-keys ## Start all services with Docker Compose
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
	cd services/auth && go mod download
	cd services/project && go mod download
	cd services/websocket && go mod download
	cd services/collaboration && go mod download
	cd services/compilation && go mod download

# ============================================
# Docker Image Build Commands
# ============================================

docker-build: ## Build Docker images for all services
	@echo "Building Docker images..."
	docker build -t texflow/auth-service:latest -f services/auth/Dockerfile services/auth
	docker build -t texflow/project-service:latest -f services/project/Dockerfile services/project
	docker build -t texflow/websocket-service:latest -f services/websocket/Dockerfile services/websocket
	docker build -t texflow/collaboration-service:latest -f services/collaboration/Dockerfile services/collaboration
	docker build -t texflow/compilation-service:latest -f services/compilation/Dockerfile services/compilation
	@echo "Docker images built successfully!"

docker-push: ## Push Docker images to registry (requires REGISTRY env var)
	@if [ -z "$(REGISTRY)" ]; then \
		echo "Error: REGISTRY environment variable not set"; \
		exit 1; \
	fi
	docker tag texflow/auth-service:latest $(REGISTRY)/texflow/auth-service:latest
	docker tag texflow/project-service:latest $(REGISTRY)/texflow/project-service:latest
	docker tag texflow/websocket-service:latest $(REGISTRY)/texflow/websocket-service:latest
	docker tag texflow/collaboration-service:latest $(REGISTRY)/texflow/collaboration-service:latest
	docker tag texflow/compilation-service:latest $(REGISTRY)/texflow/compilation-service:latest
	docker push $(REGISTRY)/texflow/auth-service:latest
	docker push $(REGISTRY)/texflow/project-service:latest
	docker push $(REGISTRY)/texflow/websocket-service:latest
	docker push $(REGISTRY)/texflow/collaboration-service:latest
	docker push $(REGISTRY)/texflow/compilation-service:latest
	@echo "Docker images pushed to $(REGISTRY)!"

# ============================================
# Kubernetes Commands
# ============================================

k8s-deploy: ## Deploy all services to Kubernetes
	@echo "Deploying to Kubernetes..."
	kubectl apply -k deployments/kubernetes/
	@echo "Deployment complete!"

k8s-delete: ## Delete all services from Kubernetes
	@echo "Deleting from Kubernetes..."
	kubectl delete -k deployments/kubernetes/ --ignore-not-found
	@echo "Deletion complete!"

k8s-status: ## Show status of Kubernetes deployments
	@echo "=== Namespace ==="
	kubectl get namespace texflow
	@echo "\n=== Pods ==="
	kubectl get pods -n texflow
	@echo "\n=== Services ==="
	kubectl get services -n texflow
	@echo "\n=== Deployments ==="
	kubectl get deployments -n texflow
	@echo "\n=== StatefulSets ==="
	kubectl get statefulsets -n texflow
	@echo "\n=== HPAs ==="
	kubectl get hpa -n texflow

k8s-logs: ## View logs for a specific service (usage: make k8s-logs SERVICE=auth-service)
	@if [ -z "$(SERVICE)" ]; then \
		echo "Usage: make k8s-logs SERVICE=<service-name>"; \
		echo "Available services: auth-service, project-service, websocket-service, collaboration-service, compilation-service"; \
		exit 1; \
	fi
	kubectl logs -n texflow -l app.kubernetes.io/name=$(SERVICE) --tail=100 -f

k8s-restart: ## Restart a specific service (usage: make k8s-restart SERVICE=auth-service)
	@if [ -z "$(SERVICE)" ]; then \
		echo "Usage: make k8s-restart SERVICE=<service-name>"; \
		exit 1; \
	fi
	kubectl rollout restart deployment/$(SERVICE) -n texflow

k8s-scale: ## Scale a service (usage: make k8s-scale SERVICE=auth-service REPLICAS=3)
	@if [ -z "$(SERVICE)" ] || [ -z "$(REPLICAS)" ]; then \
		echo "Usage: make k8s-scale SERVICE=<service-name> REPLICAS=<count>"; \
		exit 1; \
	fi
	kubectl scale deployment/$(SERVICE) -n texflow --replicas=$(REPLICAS)

k8s-port-forward: ## Port forward Kong API Gateway (usage: make k8s-port-forward)
	@echo "Port forwarding Kong to localhost:8000..."
	kubectl port-forward -n texflow svc/kong 8000:8000

k8s-describe: ## Describe a specific pod or service (usage: make k8s-describe RESOURCE=pod/auth-service-xxx)
	@if [ -z "$(RESOURCE)" ]; then \
		echo "Usage: make k8s-describe RESOURCE=<resource-type/resource-name>"; \
		echo "Example: make k8s-describe RESOURCE=pod/auth-service-xxx"; \
		exit 1; \
	fi
	kubectl describe -n texflow $(RESOURCE)
