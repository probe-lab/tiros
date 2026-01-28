#!/bin/bash

# Load common test utilities
source "$(dirname "${BASH_SOURCE[0]}")/test_common.sh"

# Setup test environment
setup_test_env chrome

echo "Waiting for chrome to be reachable..."
MAX_ATTEMPTS=20
COUNT=0
until curl -s -f http://127.0.0.1:3000/json/version > /dev/null || [ $COUNT -eq $MAX_ATTEMPTS ]; do
  sleep 0.5
  ((COUNT++))
done

if [ $COUNT -eq $MAX_ATTEMPTS ]; then
  echo "Timeout reached: Chrome not reachable."
  exit 1
fi

# Random controlled CID from ../pkg/controlledcids
STATIC_CIDS=QmSuhPykVHVHDtfXVdtksdcW9j3itxqMPdg4vEotnZfWuj

# Run tiros with a single iteration and the JSON output option
go run ./cmd/tiros probe \
  --json.out $TEMP_DIR serviceworker \
  --iterations.max 1 \
  --cids $STATIC_CIDS \
  --controlled.cids=false

# Find the service worker probes output file
OUTPUT_FILE=$(find "$TEMP_DIR" -type f -name 'service_worker_probes.ndjson' | head -n 1)

# Assert total number of probes (1 probe per iteration)
EXPECTED_PROBES=1
ACTUAL_PROBES=$(wc -l < "$OUTPUT_FILE" | tr -d ' ')
[ "$ACTUAL_PROBES" -eq "$EXPECTED_PROBES" ] || { echo " ❌ Expected $EXPECTED_PROBES probes, got $ACTUAL_PROBES"; exit 1; }
echo " ✅ Probe count matches expected $EXPECTED_PROBES"

# Parse JSON output (first line)
parse_json_output "$OUTPUT_FILE"

# Assertions on the first probe result
echo "Asserting first probe..."
assert_not_empty "RunID"
assert_eq "Gateway" "inbrowser.link"
assert_eq "CID" "$STATIC_CIDS"
assert_eq "CIDSource" "static"
assert_not_empty "URL"
assert_gt "TotalTTFBS" "0"
assert_gt "FinalTTFBS" "0"
assert_gt "TimeToFinalRedirectS" "0"
assert_not_empty "ServiceWorkerVersion"
assert_eq "StatusCode" "200"
assert_not_empty "ContentType"
assert_gt "ContentLength" "0"
assert_not_empty "IPFSPath"
assert_not_empty "IPFSRoots"
assert_eq "Error" "null"
assert_not_empty "CreatedAt"

# Success message
echo "All validations passed successfully!"
