#!/bin/bash

# Common utilities for test scripts

# Setup test environment
setup_test_env() {
    test_env="$1"

    # the directory of the calling script
    DIR="$( cd "$( dirname "${BASH_SOURCE[1]}" )" && pwd )"

    # the temp directory used, within $DIR
    # omit the -p parameter to create a temporal directory in the default location
    TEMP_DIR=$(mktemp -d -p "$DIR")
    echo "Created temporary directory $TEMP_DIR"

    # start containers in the background
    docker compose -f "$DIR/../docker-compose.$test_env.yml" up -d

    # deletes the temp directory
    function cleanup {
        docker compose -f "$DIR/../docker-compose.$test_env.yml" down -v
        docker compose -f "$DIR/../docker-compose.$test_env.yml" rm
    }

    trap cleanup EXIT
}

# Parse JSON output and set FIRST_LINE variable
parse_json_output() {
    local output_file="$1"
    # Read the first line of the NDJSON file
    FIRST_LINE=$(head -n 1 "$output_file" | jq '.')
}

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