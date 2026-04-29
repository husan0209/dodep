.PHONY: build run test clean lint docker-build docker-run migrate-up migrate-down

# Variables
BINARY_NAME=payment-service
DOCKER_IMAGE=payment-service
GO=go
GOFLAGS=-v

# Build the binary
build:
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME) ./cmd/server

# Run the service locally
run:
	$(GO) run ./cmd/server

# Run all tests
test:
	$(GO) test $(GOFLAGS) ./...

# Run tests with coverage
test-coverage:
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Run integration tests
test-integration:
	$(GO) test $(GOFLAGS) -tags=integration ./tests/integration/...

# Run property-based tests
test-property:
	$(GO) test $(GOFLAGS) -tags=property ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Lint code
lint:
	golangci-lint run ./...

# Format code
fmt:
	$(GO) fmt ./...

# Vet code
vet:
	$(GO) vet ./...

# Docker build
docker-build:
	docker build -t $(DOCKER_IMAGE):latest .

# Docker run
docker-run:
	docker run -p 8080:8080 --env-file .env $(DOCKER_IMAGE):latest

# Database migrations up (centralised in libs/migrations/postgresql)
migrate-up:
	migrate -path ../../../libs/migrations/postgresql -database "postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable" up

# Database migrations down
migrate-down:
	migrate -path ../../../libs/migrations/postgresql -database "postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable" down

# Generate mocks
mocks:
	mockgen -source=internal/repository/interfaces.go -destination=internal/mocks/repository_mock.go -package=mocks

# Install development tools
tools:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/golang/mock/mockgen@latest
	$(GO) install github.com/swaggo/swag/cmd/swag@latest

# Check for vulnerabilities
vulncheck:
	govulncheck ./...

# All checks before commit
pre-commit: fmt vet lint test
