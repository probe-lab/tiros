#!/bin/bash

# Load common test utilities
source "$(dirname "${BASH_SOURCE[0]}")/test_common.sh"

# Setup test environment
setup_test_env website

# Run tiros with a single iteration and the JSON output option
go run . probe --json.out "$TEMP_DIR" websites --websites probelab.io --probes 1 --chrome.kubo.host kubo

# Find the file suffixed with _upload.ndjson in the temp directory
OUTPUT_FILE=$(find "$TEMP_DIR" -type f -name 'website_probes.ndjson' | head -n 1)

# Parse JSON output
parse_json_output "$OUTPUT_FILE"

# Assertions on fields
echo "Asserting..."
assert_eq "KuboVersion" "0.38.2"
assert_not_empty "RunID"
assert_not_empty "KuboPeerID"
assert_not_empty "Website"
assert_not_empty "URL"
assert_not_empty "TTFBRating"
assert_not_empty "CLSRating"
assert_not_empty "FCPRating"
assert_not_empty "LCPRating"
assert_eq "Protocol" "IPFS"
assert_eq "IPFSImpl" "KUBO"
assert_eq "Try" "0"
assert_gt "TTFB" "0"
assert_gt "FCP" "0"
assert_gt "LCP" "0"
assert_gt "TTI" "0"
assert_gt "CLS" "0"
assert_gt "StatusCode" "0"

# Success message
echo "All validations passed successfully!"

