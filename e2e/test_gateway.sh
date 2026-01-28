#!/bin/bash

# Load common test utilities
source "$(dirname "${BASH_SOURCE[0]}")/test_common.sh"

# Setup test environment
setup_test_env

# Random controlled CID from ../pkg/controlledcids
STATIC_CIDS=QmSuhPykVHVHDtfXVdtksdcW9j3itxqMPdg4vEotnZfWuj

# Run tiros with a single iteration and the JSON output option
go run ./cmd/tiros probe \
  --json.out $TEMP_DIR gateways \
  --gateways ipfs.io,dweb.link \
  --iterations.max 1 \
  --cids $STATIC_CIDS \
  --controlled.cids=false

# Find the gateway probes output file
OUTPUT_FILE=$(find "$TEMP_DIR" -type f -name 'gateway_probes.ndjson' | head -n 1)

# Assert total number of probes
# Each gateway is probed with 2 formats (none, car) × 2 passes (uncached, cached)
NUM_GATEWAYS=2
PROBES_PER_GATEWAY=4
EXPECTED_PROBES=$((NUM_GATEWAYS * PROBES_PER_GATEWAY))
ACTUAL_PROBES=$(wc -l < "$OUTPUT_FILE" | tr -d ' ')
[ "$ACTUAL_PROBES" -eq "$EXPECTED_PROBES" ] || { echo " ❌ Expected $EXPECTED_PROBES probes, got $ACTUAL_PROBES"; exit 1; }
echo " ✅ Probe count matches expected $EXPECTED_PROBES"

# Parse JSON output (first line only)
parse_json_output "$OUTPUT_FILE"

# Assertions on the first probe result
echo "Asserting first probe..."
assert_not_empty "RunID"
assert_not_empty "Gateway"
assert_eq "CID" "$STATIC_CIDS"
assert_eq "CIDSource" "static"
assert_not_empty "Format"
assert_not_empty "RequestStart"
assert_gt "DNSDurationS" "0"
assert_gt "ConnDurationS" "0"
assert_gt "TTFBS" "0"
assert_gt "DownloadDurationS" "0"
assert_gt "BytesReceived" "0"
assert_gt "DownloadSpeedMbps" "0"
assert_eq "StatusCode" "200"
assert_not_empty "IPFSPath"
assert_not_empty "IPFSRoots"
assert_not_empty "ContentType"
assert_not_empty "FinalURL"
assert_eq "Error" "null"
assert_not_empty "CreatedAt"

# Success message
echo "All validations passed successfully!"
