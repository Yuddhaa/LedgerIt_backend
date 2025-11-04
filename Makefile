.PHONY: all dev up down logs build install-air
DB_DSN := "postgres://youruser:yourpass@localhost:5432/ledgerit_db?sslmode=disable"
MIGRATIONS_DIR := "sql/migrations"

# Default command
all: help

# Run the app in development mode with hot-reload
dev: #install-air .air.toml
	@echo "Starting server with hot-reload (Air)..."
	@air

# Start all Docker containers (for frontend dev or testing)
up:
	@echo "Starting Docker containers (API + DB)..."
	@docker compose up -d

# Stop and remove all Docker containers
down:
	@echo "Stopping and removing Docker containers..."
	@docker compose down

# Follow the logs for the API container
logs:
	@echo "Following API logs..."
	@docker compose logs -f api

# Build the Go binary locally
build:
	@echo "Building Go binary..."
	@mkdir -p bin
	@go build -o ./bin/server ./cmd/api/main.go

# Install the 'air' hot-reload tool
install-air:
	@echo "Installing/Updating 'air'..."
	@go install github.com/air-verse/air@latest


# Run sqlc to generate Go code
sqlc_generate:
	@echo "Generating sqlc Go code..."
	@sqlc generate

# Apply all pending migrations
migrate-up:
	@echo "Running database migrations..."
	@goose -dir ${MIGRATIONS_DIR} postgres "${DB_DSN}" up

# Roll back the last migration
migrate-down:
	@echo "Rolling back last database migration..."
	@goose -dir ${MIGRATIONS_DIR} postgres "${DB_DSN}" down

# Create a new blank migration file
migrate-new:
	@read -p "Enter migration name (e.g., add_new_column): " name; \
	goose -dir ${MIGRATIONS_DIR} create $$name sql
# Show help menu
help:
	@echo "Available commands:"
	@echo "  make dev          - Start local dev server with hot-reload"
	@echo "  make up           - Start Docker containers (API + DB) in detached mode"
	@echo "  make down         - Stop and remove Docker containers"
	@echo "  make logs         - Tail the logs from the 'api' container"
	@echo "  make build        - Build the Go binary locally"
	@echo "  make install-air  - Install the 'air' tool"
