# Makefile for Opus Casino

.PHONY: help install dev build test lint format clean docker-up docker-down proto-generate proto-lint

# Default target
help:
	@echo "Opus Casino - Available commands:"
	@echo ""
	@echo "  install         - Install all dependencies"
	@echo "  dev             - Start all services in development mode"
	@echo "  build           - Build all projects"
	@echo "  test            - Run all tests"
	@echo "  lint            - Run linters"
	@echo "  format          - Format code"
	@echo "  clean           - Clean build artifacts"
	@echo "  docker-up       - Start Docker Compose services"
	@echo "  docker-down     - Stop Docker Compose services"
	@echo "  k8s-apply       - Apply Kubernetes manifests"
	@echo "  tf-plan         - Run Terraform plan"
	@echo "  tf-apply        - Run Terraform apply"
	@echo "  proto-generate  - Generate Protobuf code"
	@echo "  proto-lint      - Lint Protobuf files"
	@echo "  proto-breaking  - Check Protobuf breaking changes"
	@echo ""

# Install all dependencies
install:
	@echo "Installing Node.js dependencies..."
	npm install
	@echo "Installing Rust dependencies..."
	cd services/rust && cargo fetch
	@echo "Installing Go dependencies..."
	@for dir in services/go/*/; do \
		echo "  Installing $$dir..."; \
		(cd "$$dir" && go mod download); \
	done
	@echo "Installing Python dependencies..."
	cd services/python/fraud-ml && pip install -e .
	cd services/python/analytics && pip install -e .

# Start development
dev:
	nx run-many --target=serve --all

# Build all projects
build:
	nx run-many --target=build --all

# Run tests
test:
	nx run-many --target=test --all

# Run linters
lint:
	nx run-many --target=lint --all

# Format code
format:
	nx format:write

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf node_modules/.cache
	rm -rf .nx/cache
	rm -rf services/rust/target
	rm -rf services/go/*/coverage
	rm -rf services/python/*/.pytest_cache
	rm -rf services/python/*/dist
	rm -rf services/python/*/build

# Docker Compose — development (infra only)
docker-up:
	docker compose -f infra/docker/docker-compose.dev.yml up -d

docker-down:
	docker compose -f infra/docker/docker-compose.dev.yml down

docker-logs:
	docker compose -f infra/docker/docker-compose.dev.yml logs -f

# Docker Compose — production (all services)
prod-up:
	docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production up -d

prod-down:
	docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production down

prod-logs:
	docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production logs -f

prod-ps:
	docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production ps

prod-pull:
	docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production pull

prod-build:
	docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production build --parallel

prod-restart:
	docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production restart

prod-migrate:
	@echo "Running PostgreSQL migrations..."
	bash tools/migrate.sh --env production
	@echo "Migrations complete."

prod-seed:
	@echo "Seeding reference data..."
	bash tools/seed.sh --env production
	@echo "Seed complete."

prod-backup:
	@echo "Running manual backup..."
	docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production exec backup /scripts/postgres-backup.sh
	@echo "Backup complete."

smoke:
	@echo "Running smoke tests..."
	bash tools/smoke-test.sh
	@echo "Smoke tests complete."

# Kubernetes
k8s-apply:
	kubectl apply -f infra/k8s/namespace.yaml
	kubectl apply -f infra/k8s/configmap.yaml
	kubectl apply -f infra/k8s/betting-engine.yaml
	kubectl apply -f infra/k8s/wallet-core.yaml
	kubectl apply -f infra/k8s/auth.yaml

# Terraform
tf-plan:
	cd infra/terraform && terraform plan -out=tfplan

tf-apply:
	cd infra/terraform && terraform apply tfplan

# Security scanning
security-scan:
	@echo "Running Trivy..."
	trivy fs .
	@echo "Running Semgrep..."
	semgrep --config auto .

# Generate Protobuf (Linux/macOS)
proto-generate: ## Generate Protobuf code for all languages
	@echo "Generating Protobuf code..."
	cd libs/proto && buf generate
	@echo "Protobuf generation complete!"

# Lint Protobuf files
proto-lint: ## Lint Protobuf files
	cd libs/proto && buf lint

# Check Protobuf breaking changes
proto-breaking: ## Check Protobuf breaking changes against main
	cd libs/proto && buf breaking --against '.git#branch=main,subdir=libs/proto'

# Windows PowerShell alternatives (no make required):
#   pwsh -File tools\proto-gen.ps1 all
#   pwsh -File tools\dev-payment.ps1
#   pwsh -File tools\dev-start.ps1

# Install Rust protoc plugins
proto-install-rust: ## Install Rust protoc plugins
	@echo "Installing Rust protoc plugins..."
	cargo install protoc-gen-prost
	cargo install protoc-gen-tonic
	@echo "Rust plugins installed!"
