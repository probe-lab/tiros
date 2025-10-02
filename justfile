#!/usr/bin/env just --justfile

# Default recipe to display available recipes
default:
    @just --list

# Install required Go tools
tools:
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.15.2
    go install github.com/aarondl/sqlboiler/v4@v4.14.1
    go install github.com/aarondl/sqlboiler/v4/drivers/sqlboiler-psql@v4.14.1

# Start PostgreSQL database in Docker
database:
    docker run --rm -p 5432:5432 -e POSTGRES_PASSWORD=password -e POSTGRES_USER=tiros_test -e POSTGRES_DB=tiros_test --name tiros_test postgres:14

# Generate database models using SQLBoiler
models:
    sqlboiler --no-tests psql

# Run database migrations up
migrate-up:
    migrate -database 'postgres://tiros_test:password@localhost:5432/tiros_test?sslmode=disable' -path migrations up

# Run database migrations down
migrate-down:
    migrate -database 'postgres://tiros_test:password@localhost:5432/tiros_test?sslmode=disable' -path migrations down

# Start all development services
dev-up:
    docker compose up -d

# Stop all development services
dev-down:
    docker compose down

# Build the Go application
build:
    go build -o tiros .

# Run the application locally
run: dev-up
    source .env.local && go run . run

# Clean up built artifacts
clean:
    rm -f tiros

# Format Go code
fmt:
    go fmt ./...

test:
    go test ./...

# Connect to local database
db-connect:
    docker exec -it tiros-db-1 psql -U tiros_test -d tiros_test