#!/bin/bash

# the directory of the script
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# the temp directory used, within $DIR
# omit the -p parameter to create a temporal directory in the default location
TEMP_DIR=$(mktemp -d -p "$DIR")
echo "Created temporary directory $TEMP_DIR"

# start kubo in the background
docker compose -f "$DIR/docker-compose.kubo.yml" up -d

# deletes the temp directory
function cleanup {
  docker compose -f "$DIR/docker-compose.kubo.yml" down -v
  docker compose -f "$DIR/docker-compose.kubo.yml" rm
}

# always kill kubo and remove the temporary directory
trap cleanup EXIT

# Run tiros with a single iteration and the JSON output option
go run . probe --json.out $TEMP_DIR kubo --iterations.max 1 --download.only --traces.receiver.host 0.0.0.0 --download.cids bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi

# Find the file suffixed with _upload.ndjson in the temp directory
OUTPUT_FILE=$(find "$TEMP_DIR" -type f -name 'downloads.ndjson' | head -n 1)

# Read the first line of the NDJSON file
FIRST_LINE=$(head -n 1 "$OUTPUT_FILE" | jq '.')

# Helper for assertions
assert_eq() {
  local field="$1"
  local actual=$(echo "$FIRST_LINE" | jq -r '.'$field)
  local expected="$2"
  [ "$actual" == "$expected" ] || { echo " ❌Assertion failed: $field (expected: $expected, got: $actual)"; exit 1; }
  echo " ✅.$field matches expected $expected"
}

assert_gt() {
  local field="$1"
  local actual=$(echo "$FIRST_LINE" | jq -r '.'$field)
  local threshold="$2"
  (( $(echo "$actual > $threshold" | bc -l) )) || { echo " ❌Assertion failed: $field (expected > $threshold, got: $actual)"; exit 1; }
  echo " ✅.$field is greater than $threshold"
}

assert_not_empty() {
  local field="$1"
  local value=$(echo "$FIRST_LINE" | jq -r '.'$field)
  if [ -z "$value" ] || [ "$value" == "null" ]; then
    echo " ❌Assertion failed: $field is empty or null"
    exit 1
  fi
  echo " ✅.$field is not empty or null"
}

# Assertions on fields
echo "Asserting..."
assert_not_empty "RunID"
assert_eq "KuboVersion" "0.38.0"
#assert_not_empty "TirosVersion"
assert_not_empty "KuboPeerID"
assert_gt "FileSizeB" "0" "Filesize is not greater than 0"
assert_not_empty "CID"
assert_not_empty "IPFSCatStart"
assert_gt "IPFSCatDurationS" "0"

# Success message
echo "All validations passed successfully!"

