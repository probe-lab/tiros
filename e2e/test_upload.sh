#!/bin/bash

# Load common test utilities
source "$(dirname "${BASH_SOURCE[0]}")/test_common.sh"

# Setup test environment
setup_test_env kubo

# Run tiros with a single iteration and the JSON output option
go run . probe --json.out $TEMP_DIR kubo --iterations.max 1 --upload.only --traces.receiver.host 0.0.0.0

# Find the file suffixed with _upload.ndjson in the temp directory
OUTPUT_FILE=$(find "$TEMP_DIR" -type f -name 'uploads.ndjson' | head -n 1)

# Parse JSON output
parse_json_output "$OUTPUT_FILE"

# Assertions on fields
echo "Asserting..."
assert_not_empty "RunID"
assert_eq "KuboVersion" "0.38.2"
assert_gt "FileSizeB" "0" "Filesize is not greater than 0"
assert_gt "IPFSAddDurationS" "0"
assert_gt "ProvideDurationS" "0"
assert_not_empty "ProvideDelayS"
assert_gt "UploadDurationS" "0"
assert_not_empty "RunID"
# assert_not_empty "TirosVersion" # not set with `go run`
assert_not_empty "KuboPeerID"
assert_not_empty "CID"
assert_not_empty "IPFSAddStart"
assert_not_empty "ProvideStart"

# Success message
echo "All validations passed successfully!"

