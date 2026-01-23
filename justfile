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

test:
    go test ./...

e2e case:
    ./e2e/test_{{case}}.sh

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
