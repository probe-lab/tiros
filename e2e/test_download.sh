#!/bin/bash

# Load common test utilities
source "$(dirname "${BASH_SOURCE[0]}")/test_common.sh"

# Setup test environment
setup_test_env kubo

STATIC_CIDS=bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi

# Run tiros with a single iteration and the JSON output option
go run . probe --json.out $TEMP_DIR kubo --iterations.max 1 --download.only --traces.receiver.host 0.0.0.0 --download.cids $STATIC_CIDS

# Find the file suffixed with _upload.ndjson in the temp directory
OUTPUT_FILE=$(find "$TEMP_DIR" -type f -name 'downloads.ndjson' | head -n 1)

# Parse JSON output
parse_json_output "$OUTPUT_FILE"

# Assertions on fields
echo "Asserting..."
assert_not_empty "RunID"
assert_eq "KuboVersion" "0.38.2"
# assert_not_empty "TirosVersion" # not set with `go run`
assert_not_empty "KuboPeerID"
assert_gt "FileSizeB" "0" "Filesize is not greater than 0"
assert_not_empty "CID"
assert_not_empty "IPFSCatStart"
assert_gt "IPFSCatDurationS" "0"

# Success message
echo "All validations passed successfully!"

