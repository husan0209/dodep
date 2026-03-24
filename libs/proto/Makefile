# Makefile for Protobuf code generation
# Usage: make gen, make lint, make clean

.PHONY: help gen gen-go gen-rust gen-ts lint breaking clean buf-update

PROTO_DIR := $(shell pwd)
GEN_DIR := $(PROTO_DIR)/gen
BUF := buf

help: ## Show this help
	@echo "Available targets:"
	@echo "  gen         - Generate code for all languages"
	@echo "  gen-go      - Generate Go code only"
	@echo "  gen-rust    - Generate Rust code only"
	@echo "  gen-ts      - Generate TypeScript code only"
	@echo "  lint        - Run buf lint"
	@echo "  breaking    - Check for breaking changes"
	@echo "  buf-update  - Update buf dependencies"
	@echo "  clean       - Remove generated code"

# Check if buf is installed
check-buf:
	@command -v $(BUF) >/dev/null 2>&1 || { echo >&2 "buf is not installed. Install from https://docs.buf.build/installation"; exit 1; }

# Generate code for all languages
gen: check-buf ## Generate code for all languages
	@echo "Generating code..."
	$(BUF) generate
	@echo "Code generation complete!"
	@echo "Generated code:"
	@echo "  - Go:         $(GEN_DIR)/go/"
	@echo "  - Rust:       $(GEN_DIR)/rust/"
	@echo "  - TypeScript: $(GEN_DIR)/typescript/"

# Generate Go code only
gen-go: check-buf ## Generate Go code only
	@echo "Generating Go code..."
	$(BUF) generate --template buf.gen.yaml --output $(GEN_DIR)/go
	@echo "Go code generation complete!"

# Generate Rust code only (requires local plugins)
gen-rust: check-buf ## Generate Rust code only
	@echo "Generating Rust code..."
	@echo "Note: Requires protoc-gen-prost and protoc-gen-tonic"
	$(BUF) generate --template buf.gen.yaml --output $(GEN_DIR)/rust
	@echo "Rust code generation complete!"

# Generate TypeScript code only
gen-ts: check-buf ## Generate TypeScript code only
	@echo "Generating TypeScript code..."
	$(BUF) generate --template buf.gen.yaml --output $(GEN_DIR)/typescript
	@echo "TypeScript code generation complete!"

# Run buf lint
lint: check-buf ## Run buf lint
	@echo "Running buf lint..."
	$(BUF) lint
	@echo "Lint complete!"

# Check for breaking changes against main branch
breaking: check-buf ## Check for breaking changes
	@echo "Checking for breaking changes against main..."
	$(BUF) breaking --against '.git#branch=main,subdir=libs/proto'
	@echo "No breaking changes detected!"

# Update buf dependencies
buf-update: check-buf ## Update buf dependencies
	@echo "Updating buf dependencies..."
	$(BUF) dep update
	@echo "Dependencies updated!"

# Clean generated code
clean: ## Remove generated code
	@echo "Cleaning generated code..."
	rm -rf $(GEN_DIR)
	@echo "Clean complete!"

# Install Rust protoc plugins (for local codegen)
install-rust-plugins: ## Install Rust protoc plugins
	@echo "Installing Rust protoc plugins..."
	cargo install protoc-gen-prost
	cargo install protoc-gen-tonic
	@echo "Rust plugins installed!"

# Default target
all: gen lint
