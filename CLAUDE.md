# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Tiros is an IPFS website measurement tool designed to measure the performance of websites accessed through IPFS gateways compared to HTTP. It runs as a scheduled AWS ECS task across multiple regions and measures web performance metrics including Time to First Byte (TTFB), First Contentful Paint (FCP), Largest Contentful Paint (LCP), Cumulative Layout Shift (CLS), and Time to Interactive (TTI).

The system consists of three main components:
1. **scheduler** (this Go application) - orchestrates measurements
2. **chrome** - headless browser via browserless/chrome
3. **ipfs** - IPFS implementation (kubo or helia-http-gateway)

## Architecture Overview

**Core Components:**
- `cmd.go` / `cmd_run.go` - CLI interface and main run command logic
- `probe.go` - Website probing and measurement logic using go-rod for browser automation
- `db_client.go` - Database operations and connections
- `models/` - SQLBoiler-generated database models (runs, measurements, providers)
- `migrations/` - Database schema migrations
- `js/` - JavaScript files injected into browser for performance measurement
- `rod/` - Browser automation wrapper utilities

**Measurement Flow:**
1. Load websites and settle for configured time
2. For each website, measure both IPFS (`/ipns/<website>`) and HTTP (`https://<website>`) variants
3. Use headless Chrome with cache-clearing techniques and performance monitoring
4. Inject TTI polyfill and web-vitals JavaScript for metrics collection
5. Store results in PostgreSQL database
6. Run garbage collection on IPFS node between requests

## Development Commands

### Local Development Setup
```bash
# Start all required services (PostgreSQL, IPFS kubo, headless Chrome)
docker compose up -d

# Set environment variables for local development
source .env.local

# Build and run
go build -o tiros .
./tiros run

# OR run directly
go run . run
```

### Database Operations
```bash
# Install required tools
make tools

# Generate database models from schema (after migration changes)
make models

# Run database migrations
make migrate-up

# Rollback migrations
make migrate-down

# Create new migration
migrate create -ext sql -dir migrations -seq create_new_table
```

### Local Database Access
```bash
# Connect to local PostgreSQL instance
docker exec -it tiros-db-1 psql -U tiros_test -d tiros_test
```

### Building and Deployment
```bash
# Build Docker image for AWS ECS
make docker

# Push to ECR
make docker-push
```

## Configuration

The application uses CLI flags with environment variable overrides (TIROS_* prefix). Key parameters:
- `--websites` - Comma-separated list of websites to test
- `--region` - AWS region identifier for this measurement run
- `--settle-times` - Wait times between measurement rounds (default: 10s, 1200s)
- `--times` - Number of measurements per website per round (default: 3)
- Database connection parameters (`--db-*`)
- Service ports for IPFS API/Gateway and Chrome CDP

## Code Patterns

**Database Operations:** Uses SQLBoiler ORM with generated models. All database operations go through `db_client.go` with proper connection pooling and transaction handling.

**Browser Automation:** Uses go-rod library with specific cache-clearing patterns:
1. Incognito browser sessions
2. Cookie clearing
3. localStorage.clear() via injected JavaScript
4. Network cache disabling via Chrome DevTools Protocol

**JavaScript Injection:** Performance measurement relies on injecting:
- `onNewDocument.js` - Sets up performance observers and clears cache
- `tti-polyfill.js` - Time to Interactive measurement
- `web-vitals.iife.js` - Core Web Vitals measurement
- `measurement.js` - Orchestrates all measurements and returns JSON

**Error Handling:** Uses structured logging via logrus. Database errors are handled with rollbacks, and browser automation uses go-rod's Try() pattern for graceful failure recovery.

## Testing

No explicit test framework is configured. For manual testing:
1. Use `docker compose up -d` to start dependencies
2. Use `.env.local` for local configuration
3. Test individual websites with minimal configuration
4. Check database for stored measurements
5. Monitor logs for debugging browser automation issues

## Key Files to Understand

- `probe.go:712` - Main website measurement logic
- `js/measurement.js` - Browser-side performance measurement script
- `cmd_run.go` - CLI configuration and measurement orchestration loop
- `models/measurements.go` - Database schema for measurement data
- `migrations/` - Database schema evolution history