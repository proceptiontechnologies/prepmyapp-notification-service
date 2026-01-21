# Makefile for notification
# Run `make help` to see available commands

.PHONY: build run dev test lint clean help

# Default target
.DEFAULT_GOAL := help

# Build the binary
build:
	@echo "Building notification..."
	go build -o bin/server ./cmd/server

# Run the server (production mode)
run: build
	@echo "Running notification..."
	./bin/server

# Run with hot reload (requires air: go install github.com/air-verse/air@latest)
dev:
	@echo "Starting development server with hot reload..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "air not installed. Install with: go install github.com/air-verse/air@latest"; \
		echo "Falling back to regular run..."; \
		go run ./cmd/server; \
	fi

# Run tests
test:
	@echo "Running tests..."
	go test -v -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  brew install golangci-lint"; \
		exit 1; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Database migrations (requires golang-migrate)
migrate-up:
	@echo "Running migrations up..."
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	@echo "Running migrations down..."
	migrate -path migrations -database "$$DATABASE_URL" down 1

migrate-create:
	@echo "Creating new migration..."
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

# Generate Swagger docs (requires swag)
swagger:
	@echo "Generating Swagger docs..."
	@if command -v swag > /dev/null; then \
		swag init -g cmd/server/main.go -o api; \
	else \
		echo "swag not installed. Install with: go install github.com/swaggo/swag/cmd/swag@latest"; \
	fi

# Help
help:
	@echo "notification Makefile commands:"
	@echo ""
	@echo "  make build          - Build the binary"
	@echo "  make run            - Build and run the server"
	@echo "  make dev            - Run with hot reload (requires air)"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make lint           - Run linter (requires golangci-lint)"
	@echo "  make fmt            - Format code"
	@echo "  make tidy           - Tidy dependencies"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make migrate-up     - Run database migrations"
	@echo "  make migrate-down   - Rollback last migration"
	@echo "  make migrate-create - Create a new migration"
	@echo "  make swagger        - Generate Swagger docs"
	@echo "  make help           - Show this help"
