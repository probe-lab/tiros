# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Tiros** is an IPFS Kubo performance measurement tool developed by ProbeLab. It performs systematic measurements of IPFS network performance across two measurement modes:

1. **Content Routing Performance** - Upload/download measurements via OpenTelemetry trace analysis
2. **Website Performance** - IPFS gateway performance for serving websites via IPNS using browser automation

**Technology Stack:**
- Go 1.25.1
- urfave/cli v3 (CLI framework)
- ClickHouse (time-series database)
- OpenTelemetry (distributed tracing)
- go-rod (Chrome DevTools Protocol automation)
- libp2p and IPFS Kubo integration

## Development Commands

### Building and Testing
```bash
# Build the binary
just build
go build -o tiros .

# Run unit tests
just test
go test ./...

# Run unit tests with race detector
go test -race ./...

# Format code
just fmt
go fmt ./...

# Clean build artifacts
just clean
```

### End-to-End Testing
```bash
# Run E2E tests (requires Docker)
just e2e upload      # Test content upload to Kubo
just e2e download    # Test content download from Kubo
just e2e website     # Test website performance measurement
```

E2E tests use docker-compose to orchestrate Kubo, Chrome, and Tiros instances. Tests validate:
- JSON output format correctness
- Expected metrics are present and pass thresholds
- Integration between components (Kubo RPC, OTLP traces, Chrome DevTools)

### Running Locally

**Content Routing (Upload/Download):**
```bash
# Terminal 1: Start Kubo with trace receiver
docker compose -f docker-compose.kubo.yml up

# Terminal 2: Run measurement
go run . probe --json.out out kubo \
  --iterations.max 1 \
  --traces.receiver.host 0.0.0.0 \
  --download.cids bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi
```

**Website Performance:**
```bash
# Terminal 1: Start Kubo + Chrome
docker compose -f docker-compose.website.yml up

# Terminal 2: Run website probe
go run . probe --json.out out websites \
  --websites ipfs.io,docs.libp2p.io \
  --probes 3
```

**Debug with Jaeger:**
```bash
# Start Jaeger
docker run --rm --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 55680:4317 \
  cr.jaegertracing.io/jaegertracing/jaeger:2.11.0

# Forward Kubo traces to Jaeger
go run . probe --json.out out kubo \
  --traces.forward.host 127.0.0.1 \
  --traces.forward.port 55680 \
  --traces.receiver.host 0.0.0.0
```

### Database Migrations
```bash
# Install migration tool
just tools

# Create new migration
migrate create -dir migrations -ext sql -seq create_new_table

# Apply migrations (production)
migrate -database 'clickhouse://clickhouse_host:9440?username=tiros&password=$PASSWORD&database=tiros&secure=true' \
  -path migrations up
```

## High-Level Architecture

### Core Design Pattern

Tiros operates as a **three-layer measurement platform**:

```
CLI Layer (urfave/cli)
    ↓
Probe Executors (kubo.go, website_probe.go)
    ↓
Data Collectors & Persistence
    ├── OpenTelemetry Trace Receiver (gRPC on :4317)
    ├── Kubo RPC Client (HTTP API :5001)
    ├── Chrome DevTools Protocol Client (:3000)
    └── Database Abstraction (ClickHouse/JSON/Noop)
```

### Measurement Flows

**Content Routing (Upload):**
1. Tiros generates random data → Kubo RPC `ipfs add`
2. Kubo emits OpenTelemetry traces → Tiros trace receiver
3. Tiros parses traces to extract provider record publishing timing
4. Metrics stored: `ipfs_add_duration_s`, `provide_duration_s`, `provide_delay_s`
5. Data persisted to ClickHouse `uploads` table

**Content Routing (Download):**
1. Tiros → Kubo RPC `ipfs cat <cid>`
2. Kubo discovers providers via DHT, emits traces
3. Tiros parses traces for provider discovery events
4. Metrics stored: `found_prov_count`, `first_prov_conn_at`, `ipfs_cat_ttfb_s`
5. Data persisted to ClickHouse `downloads` table

**Website Performance:**
1. Setup: Chrome incognito mode + clear cache/cookies/localStorage
2. Navigate to website via Kubo gateway: `http://localhost:8080/ipns/<website>`
3. Wait for page load + idle state
4. Inject JavaScript libraries: `tti-polyfill.js`, `web-vitals.iife.js`, `measurement.js`
5. Collect Core Web Vitals: TTFB, FCP, LCP, TTI, CLS
6. Run Kubo garbage collection to clear cache
7. Compare with HTTPS baseline: `https://<website>`
8. Data persisted to ClickHouse `website_probes` table

### File Organization

**Root-level Go files (all critical logic):**
- `cmd.go` - CLI setup and command registration (entry point)
- `cmd_probe.go` - Base probe configuration
- `cmd_probe_kubo.go` - Content routing probe orchestration (414 lines)
- `cmd_probe_websites.go` - Website probe orchestration (351 lines)
- `cmd_probe_gateways.go` - Gateway probe orchestration
- `kubo.go` - Kubo RPC client wrapper (upload/download ops, 512 lines)
- `website_probe.go` - Browser automation logic (260 lines)
- `trace_receiver.go` - OTLP gRPC server (222 lines)
- `trace_parse.go` - Span analysis and metric extraction (240 lines)
- `db.go` - Database client abstraction (297 lines)
- `db_models.go` - Data models (UploadModel, DownloadModel, WebsiteProbeModel)
- `cid_provider.go` - CID generation/fetching utilities
- `metrics.go` - Web vitals parsing
- `js.go` - Embedded JavaScript handling

**Key directories:**
- `js/` - Browser-side JavaScript (measurement.js, tti-polyfill.js, web-vitals.iife.js)
- `migrations/` - ClickHouse schema migrations (8 migrations for uploads, downloads, website_probes, providers, etc.)
- `e2e/` - End-to-end test scripts (bash with docker-compose orchestration)

### Critical Abstraction Layers

**DBClient Interface:**
- `ClickhouseClient` - Production database writes
- `JSONClient` - Local testing (newline-delimited JSON files)
- `NoopClient` - Dry-run mode (no persistence)

**Trace Processing Pipeline:**
1. `TraceReceiver` - Collects gRPC OTLP messages from Kubo
2. `TraceMatcher` - Filters relevant spans (e.g., "ipfs/core.Bitswap.WriteProviders")
3. `trace_parse.go` - Extracts timing metrics from matched spans
4. Storage - Persists to database via DBClient interface

**Configuration System:**
- Global flags: `--log.level`, `--metrics.enabled`, `--tracing.enabled`, `--aws.region`
- Probe flags: `--dry.run`, `--json.out`, `--timeout`
- Kubo-specific: `--filesize`, `--iterations.max`, `--traces.receiver.host/port`, `--download.cids`, `--upload.only`, `--download.only`
- Website-specific: `--websites`, `--probes`, `--chrome.cdp.host/port`, `--kubo.gateway.port`

## Important Implementation Details

### OpenTelemetry Trace Handling

Tiros receives traces via gRPC OTLP on port 4317. Kubo must be configured to send traces to this endpoint:
```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://tiros:4317
```

**Key trace parsing logic** (`trace_parse.go`):
- Filters spans by name patterns (e.g., "ipfs/core.Bitswap.WriteProviders")
- Matches spans to upload/download CIDs
- Calculates timing deltas between span start/end times
- Handles trace buffering and out-of-order arrival

**Trace forwarding**: Use `--traces.forward.host/port` to duplicate traces to Jaeger/Grafana for debugging while still processing in Tiros.

### Browser Automation Cache Prevention

Website performance measurements use **four layers of cache prevention**:
1. Incognito browser mode (`browser.MustIncognito()`)
2. Clear cookies (`browser.MustSetCookies()` with empty args)
3. JavaScript localStorage clearing (`page.MustEvalOnNewDocument(jsOnNewDocument)`)
4. Network cache disabled (`proto.NetworkSetCacheDisabled{CacheDisabled: true}`)

After each IPFS request, run `repo/gc` to clear Kubo's local cache. This ensures measurements reflect true network performance, not cache hits.

### Settle Times and Warm Cache Testing

The website probe uses a "settle time" loop to measure both cold and warm cache performance:
```go
for _, settle := range c.IntSlice("settle-times") {
    time.Sleep(time.Duration(settle) * time.Second)
    for i := 0; i < c.Int("times"); i++ {
        // Measure each website via IPFS and HTTP
    }
}
```

Typical production config: `settle-times=[0, 600]` (0s = cold start, 10min = warmed up), `times=5` (5 measurements per settle time).

### Deployment Context

Production runs on AWS ECS in 4 regions: `eu-central-1`, `us-east-2`, `us-west-1`, `ap-southeast-2`.

Each ECS task contains:
- `tiros` (scheduler container)
- `browserless/chrome` (Chrome DevTools Protocol)
- `ipfs/kubo` (IPFS implementation, run with `LIBP2P_RCMGR=0` to disable resource manager)

Alternative IPFS implementations must support:
- `/api/v0/repo/gc` endpoint
- `/api/v0/version` endpoint
- `/api/v0/id` endpoint
- Basic IPFS Gateway with IPNS resolution

## Data Models and Metrics

### Upload Metrics (ClickHouse `uploads` table)
- `run_id`, `region`, `tiros_version`, `kubo_version`, `kubo_peer_id`
- `file_size_b`, `cid`
- `ipfs_add_start`, `ipfs_add_duration_s`
- `provide_start`, `provide_duration_s`, `provide_delay_s`
- `upload_duration_s`, `error`

### Download Metrics (ClickHouse `downloads` table)
- Similar identifiers as uploads
- `ipfs_cat_start`, `ipfs_cat_duration_s`, `ipfs_cat_ttfb_s`
- `found_prov_count`, `conn_prov_count`
- `first_conn_prov_found_at`, `first_prov_conn_at`
- `ipni_start`, `ipni_duration_s`, `ipni_status`
- `discovery_method`, `cid_source`

### Website Performance Metrics (ClickHouse `website_probes` table)
- `run_id`, `region`, `website`, `url`, `protocol` (IPFS/HTTP)
- Core Web Vitals:
  - `ttfb_s` - Time to First Byte
  - `fcp_s` - First Contentful Paint
  - `lcp_s` - Largest Contentful Paint
  - `tti_s` - Time to Interactive
  - `cls_s` - Cumulative Layout Shift
- Rating fields for each metric (good/needs-improvement/poor)
- `status_code`, `body`, `metrics` (JSON), `error`

## Code Style and Standards

This project follows ethPandaOps Go standards:

### CLI Framework Caveat
- Tiros uses `urfave/cli v3`, **not** the standard `github.com/spf13/cobra`
- Command structure: `tiros probe {kubo|websites|gateways}`

### Logging
- Uses `github.com/sirupsen/logrus` (already integrated via `github.com/probe-lab/go-commons`)
- Log levels: debug, info, warn, error
- Configure via `--log.level` and `--log.format` (text/json)

### Concurrency Patterns
- All I/O operations accept `context.Context` as first parameter
- Use `errgroup` for parallel operations that can fail (see trace processing)
- Goroutines must have clear lifecycle: use `sync.WaitGroup` and context cancellation
- Test with race detector: `go test -race ./...`

### Error Handling
- No errors left unchecked
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Use `errors.Is()` and `errors.As()` for error checking
- Log errors at appropriate levels

### Testing
- Use table-driven tests for multiple scenarios
- Use `testify/assert` or `testify/require` for assertions
- Aim for 70% test coverage of critical paths
- E2E tests verify integration between components

### Naming Conventions
- NO package stuttering: `user.User` not `user.UserUser`
- NO generic packages: `types/`, `utils/`, `common/`, `helpers/`
- Avoid overly long lines (soft limit: 99 characters)
- Group imports: standard library first, then external packages

## Common Gotchas

1. **Trace receiver must bind to 0.0.0.0** when running in Docker: `--traces.receiver.host 0.0.0.0` (not 127.0.0.1)
2. **Chrome requires specific host** when running in Docker: `--chrome.kubo.host` may differ from `--kubo.host` due to Docker networking
3. **Garbage collection is critical** after IPFS requests to prevent cache pollution in multi-iteration tests
4. **Trace parsing is CID-dependent**: Upload and download operations must track CIDs to match traces correctly
5. **Website probe timeout behavior**: Uses `rod.Try()` to catch panics from timeout failures and convert to errors
6. **JSON output for local testing**: Use `--json.out <dir>` to write newline-delimited JSON files instead of ClickHouse
7. **Infinite iterations**: `--iterations.max 0` runs indefinitely (production mode)
