# Task runner. `just` lists the recipes; CI runs `just ci`.

# The lint tools, pinned here so a person and CI run the same versions.
gofumpt := "go run mvdan.cc/gofumpt@v0.12.0"
staticcheck := "go run honnef.co/go/tools/cmd/staticcheck@2026.2.2"

# List the recipes
default:
    @just --list

# Install required Go tools
tools:
    go install -tags 'clickhouse' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.15.2

# Build the tiros binary
build:
    go build -o tiros ./cmd/tiros

# Clean up built artifacts
clean:
    rm -f tiros
    rm -r ./e2e/tmp.* || true

# Check formatting, go fix, vet, and staticcheck without changing files
lint:
    @test -z "$({{ gofumpt }} -l .)" || { echo "not formatted, run just format:"; {{ gofumpt }} -l .; exit 1; }
    @d=$(go fix -diff ./...) && test -z "$d" || { echo "$d"; echo "go fix changes pending, run just format"; exit 1; }
    go vet ./...
    {{ staticcheck }} ./...

# Format the code and apply go fix in place
format:
    {{ gofumpt }} -w .
    go fix ./...

# Run the tests
test *args:
    go test ./... {{ args }}

# Run everything CI runs
ci: lint build test

# Run one end-to-end test: website, upload, download, serviceworker or gateway
e2e case:
    ./e2e/test_{{case}}.sh

# Fetch recent CIDs from the bitswap sniffer into testdata/cids.txt
fetch-testcids:
    @echo "Fetching the latest 100 CIDs from the bitswap sniffer..."
    @mkdir -p testdata
    @clickhouse client --host ym2rzr065h.us-east-1.aws.clickhouse.cloud --password --user bitswap_sniffer_ipfs_ro --database bitswap_sniffer_ipfs --query "select cid from shared_cids where origin = 'dht' and msg_type = 'add-provider-records' order by timestamp desc limit 100" \
      | sort \
      | uniq \
      | tr '\n' ',' \
      | sed 's/,$//' \
      > testdata/cids.txt
    @echo "CIDs stored in ./testdata/cids.txt. Use as:"
    @echo '  --cids `cat ./testdata/cids.txt`'
