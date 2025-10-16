#!/bin/bash

# the directory of the script
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# the temp directory used, within $DIR
# omit the -p parameter to create a temporal directory in the default location
TEMP_DIR=$(mktemp -d -p "$DIR")

# start kubo in the background
nohup env \
  OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 \
  OTEL_EXPORTER_OTLP_INSECURE=true \
  OTEL_EXPORTER_OTLP_PROTOCOL=grpc \
  OTEL_SERVICE_NAME=kubo \
  OTEL_TRACES_EXPORTER=otlp \
  ipfs daemon > "$TEMP_DIR/kubo.out" 2>&1 &

# capture its process id
KUBO_PID=$!

# deletes the temp directory
function cleanup {
  kill $KUBO_PID
  rm -rf "$TEMP_DIR" || true
}

# always kill kubo and remove the temporary directory
trap cleanup EXIT

# Run tiros with a single iteration and the JSON output option
go run ./cmd/tiros probe --json-out $TEMP_DIR kubo --maxIterations 1

# Find the file suffixed with _upload.ndjson in the temp directory
OUTPUT_FILE=$(find "$TEMP_DIR" -type f -name '*_uploads.ndjson' | head -n 1)

# Read the first line of the NDJSON file
FIRST_LINE=$(head -n 1 "$OUTPUT_FILE" | jq '.')

# Helper for assertions
assert_eq() {
  local actual="$1"
  local expected="$2"
  local message="$3"
  [ "$actual" == "$expected" ] || { echo "Assertion failed: $message (expected: $expected, got: $actual)"; exit 1; }
}

assert_gt() {
  local actual="$1"
  local threshold="$2"
  local message="$3"
  (( $(echo "$actual > $threshold" | bc -l) )) || { echo "Assertion failed: $message (expected > $threshold, got: $actual)"; exit 1; }
}

# Assertions on fields
assert_eq "$(echo "$FIRST_LINE" | jq -r '.KuboVersion')" "0.38.0" "Version does not equal '0.38.0'"
assert_gt "$(echo "$FIRST_LINE" | jq -r '.FileSizeMiB')" "0" "Filesize is not greater than 0"
assert_gt "$(echo "$FIRST_LINE" | jq -r '.IPFSAddDurationMs')" "0" "IPFSAddDurationMs is not greater than 0"

# Success message
echo "All validations passed successfully!"

