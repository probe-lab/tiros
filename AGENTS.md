# tiros

ProbeLab repository. Follow the `probelab` skill: it holds the company rules. Load it before you start any task here.

Tiros measures IPFS performance from several regions. It has four probes: traditional HTTP gateways, service worker gateways (headless Chrome), Kubo retrieval and publication, and Kubo website loading. It stores results in ClickHouse. ProbeLab runs it; the results feed the ProbeLab dashboards.

## Layout

- `cmd/tiros/` — CLI entry point and one file per probe command (`probe gateways`, `probe serviceworker`, `probe kubo`, `probe websites`)
- `pkg/` — CID providers, `db/` (ClickHouse client and `migrations/`), `kubo/`, `sw/`, `ws/`, `js/` (scripts injected into Chrome)
- `e2e/` — end-to-end shell tests, run with `just e2e <case>`
- `testdata/` — Kubo trace fixtures for unit tests
- `docker-compose.yml` — local ClickHouse, Kubo and headless Chrome
- `config.alloy` — Grafana Alloy configuration

## Commands

- `just build`, `just test`, `just lint`, `just format`, `just ci` (the checks CI runs)
- `just e2e website|upload|download|serviceworker|gateway` — end-to-end tests; need Docker
- Run locally: `docker compose up clickhouse` (add `chrome` or `kubo` per probe), then `go run ./cmd/tiros probe <probe> --iterations.max 1`. The README has the flags per probe.
- `just tools` installs `migrate`. `just fetch-testcids` needs access to the bitswap sniffer ClickHouse.

## Conventions

- Migrations in `pkg/db/migrations/` are applied on startup. Write them for a clustered ClickHouse.
- Flags read environment variables with the prefix `TIROS_`. List new ones in `.env.example` and the README.
- The Docker image is built from `Dockerfile` and pushed to ECR by CI as `tiros-sha<commit>`.

## Related repositories

- bitswap-sniffer — its ClickHouse database is the source of CIDs for the probes
- probelab-infra — deploys tiros

## Owner

@probe-lab/probelab-core
