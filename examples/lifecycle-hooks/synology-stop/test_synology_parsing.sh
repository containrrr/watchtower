#!/bin/sh
# Unit tests for parsing functions in synology-stop.sh
# Tests parse_container_name() and SID/SYNOTOKEN extraction

set -eu

# ------------------------------------------------------------------
# Functions under test (fixed versions for testing)
# ------------------------------------------------------------------

parse_container_name() {
    # Extract name field from JSON using awk (fixed version)
    # Run awk and check its exit status to catch parse errors
    if ! awk '
    BEGIN { RS=""; FS="" }
    {
        # Find "name": followed by optional spaces and "
        if (match($0, /"name"[[:space:]]*:[[:space:]]*"(\\.|[^"\\])*"/)) {
            # Extract the content between the quotes
            content = substr($0, RSTART, RLENGTH)
            # Remove "name": " from start and " from end
            sub(/^"name"[[:space:]]*:[[:space:]]*"/, "", content)
            sub(/"$/, "", content)
            # Unescape quotes
            gsub(/\\"/, "\"", content)
            print content
        }
    }' 2>/dev/null; then
        echo "Error: awk failed to parse container name" >&2
        return 1
    fi
}

extract_sid() {
    # Only print if sid field exists
    sed -n '/"sid"[[:space:]]*:[[:space:]]*"[^"]*"/{ s/.*"sid"[[:space:]]*:[[:space:]]*"//; s/".*//p }'
}

extract_synotoken() {
    # Only print if synotoken field exists
    sed -n '/"synotoken"[[:space:]]*:[[:space:]]*"[^"]*"/{ s/.*"synotoken"[[:space:]]*:[[:space:]]*"//; s/".*//p }'
}

# ------------------------------------------------------------------
# Test framework
# ------------------------------------------------------------------

TEST_COUNT=0
PASS_COUNT=0
FAIL_COUNT=0

assert_equal() {
    expected="$1"
    actual="$2"
    test_name="$3"

    TEST_COUNT=$((TEST_COUNT + 1))

    if [ "$actual" = "$expected" ]; then
        echo "PASS: $test_name"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "FAIL: $test_name"
        echo "  Expected: '$expected'"
        echo "  Actual:   '$actual'"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
}

# ------------------------------------------------------------------
# Tests for parse_container_name()
# ------------------------------------------------------------------

test_parse_container_name_valid_simple() {
    input='{"name":"my-container"}'
    expected="my-container"
    actual=$(printf '%s\n' "$input" | parse_container_name)
    assert_equal "$expected" "$actual" "parse_container_name: valid simple name"
}

test_parse_container_name_valid_with_spaces() {
    input='{"name": "my-container"}'
    expected="my-container"
    actual=$(printf '%s\n' "$input" | parse_container_name)
    assert_equal "$expected" "$actual" "parse_container_name: valid name with spaces"
}

test_parse_container_name_valid_escaped_quotes() {
    input='{"name":"my\"container"}'
    expected='my"container'
    actual=$(printf '%s\n' "$input" | parse_container_name)
    assert_equal "$expected" "$actual" "parse_container_name: valid name with escaped quotes"
}

test_parse_container_name_valid_special_chars() {
    input='{"name":"my_container-123.test"}'
    expected="my_container-123.test"
    actual=$(printf '%s\n' "$input" | parse_container_name)
    assert_equal "$expected" "$actual" "parse_container_name: valid name with special characters"
}

test_parse_container_name_empty_name() {
    input='{"name":""}'
    expected=""
    actual=$(printf '%s\n' "$input" | parse_container_name)
    assert_equal "$expected" "$actual" "parse_container_name: empty name"
}

test_parse_container_name_malformed_no_name_field() {
    input='{"other":"value"}'
    expected=""
    actual=$(printf '%s\n' "$input" | parse_container_name)
    assert_equal "$expected" "$actual" "parse_container_name: no name field"
}

# ------------------------------------------------------------------
# Tests for SID extraction
# ------------------------------------------------------------------

test_extract_sid_valid() {
    input='{"success":true,"data":{"sid":"abc123","synotoken":"def456"}}'
    expected="abc123"
    actual=$(printf '%s\n' "$input" | extract_sid)
    assert_equal "$expected" "$actual" "extract_sid: valid response"
}

test_extract_sid_with_spaces() {
    input='{"success": true, "data": {"sid": "abc123", "synotoken": "def456"}}'
    expected="abc123"
    actual=$(printf '%s\n' "$input" | extract_sid)
    assert_equal "$expected" "$actual" "extract_sid: valid response with spaces"
}

test_extract_sid_missing() {
    input='{"success":true,"data":{"synotoken":"def456"}}'
    expected=""
    actual=$(printf '%s\n' "$input" | extract_sid)
    assert_equal "$expected" "$actual" "extract_sid: missing sid field"
}

test_extract_sid_empty() {
    input='{"success":true,"data":{"sid":"","synotoken":"def456"}}'
    expected=""
    actual=$(printf '%s\n' "$input" | extract_sid)
    assert_equal "$expected" "$actual" "extract_sid: empty sid value"
}

# ------------------------------------------------------------------
# Tests for SYNOTOKEN extraction
# ------------------------------------------------------------------

test_extract_synotoken_valid() {
    input='{"success":true,"data":{"sid":"abc123","synotoken":"def456"}}'
    expected="def456"
    actual=$(printf '%s\n' "$input" | extract_synotoken)
    assert_equal "$expected" "$actual" "extract_synotoken: valid response"
}

test_extract_synotoken_with_spaces() {
    input='{"success": true, "data": {"sid": "abc123", "synotoken": "def456"}}'
    expected="def456"
    actual=$(printf '%s\n' "$input" | extract_synotoken)
    assert_equal "$expected" "$actual" "extract_synotoken: valid response with spaces"
}

test_extract_synotoken_missing() {
    input='{"success":true,"data":{"sid":"abc123"}}'
    expected=""
    actual=$(printf '%s\n' "$input" | extract_synotoken)
    assert_equal "$expected" "$actual" "extract_synotoken: missing synotoken field"
}

test_extract_synotoken_empty() {
    input='{"success":true,"data":{"sid":"abc123","synotoken":""}}'
    expected=""
    actual=$(printf '%s\n' "$input" | extract_synotoken)
    assert_equal "$expected" "$actual" "extract_synotoken: empty synotoken value"
}

# ------------------------------------------------------------------
# Test runner
# ------------------------------------------------------------------

run_tests() {
    echo "Running unit tests for synology-stop.sh parsing functions..."
    echo

    # Run all test functions
    test_parse_container_name_valid_simple
    test_parse_container_name_valid_with_spaces
    test_parse_container_name_valid_escaped_quotes
    test_parse_container_name_valid_special_chars
    test_parse_container_name_empty_name
    test_parse_container_name_malformed_no_name_field

    test_extract_sid_valid
    test_extract_sid_with_spaces
    test_extract_sid_missing
    test_extract_sid_empty

    test_extract_synotoken_valid
    test_extract_synotoken_with_spaces
    test_extract_synotoken_missing
    test_extract_synotoken_empty

    echo
    echo "Test Results:"
    echo "Total tests: $TEST_COUNT"
    echo "Passed: $PASS_COUNT"
    echo "Failed: $FAIL_COUNT"

    if [ "$FAIL_COUNT" -eq 0 ]; then
        echo "All tests passed!"
        return 0
    else
        echo "Some tests failed."
        return 1
    fi
}

# ------------------------------------------------------------------
# Main execution
# ------------------------------------------------------------------

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
    echo "Unit tests for synology-stop.sh parsing functions"
    echo "Usage: $0"
    echo "Runs all tests and reports results"
    exit 0
fi

run_tests
