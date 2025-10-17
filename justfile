#!/usr/bin/env just --justfile

# Default recipe to display available recipes
default:
    @just --list

# Install required Go tools
tools:
    go install -tags 'clickhouse' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.15.2

# Build the Go application
build:
    go build -o tiros .

# Clean up built artifacts
clean:
    rm -f tiros
    rm -r ./e2e/tmp.* || true

# Format Go code
fmt:
    go fmt ./...

# Connect to local database
db-connect:
    docker exec -it tiros-db-1 psql -U tiros_test -d tiros_test

e2e case:
    ./e2e/test_{{case}}.sh

