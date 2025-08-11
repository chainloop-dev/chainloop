#!/bin/bash

# HTTP Hostname Validation Policy Tests
# This file demonstrates the --allowed-hostnames flag functionality
source ../_testutils.sh

# Initialize test framework
init_tests

# Verify required files exist
verify_files "policy.yaml" "testdata/empty.json"

# Get current platform version from the API to perform the test passing the right version
get_platform_version() {
    if command -v curl &> /dev/null; then
        curl -s https://app.chainloop.dev/api/info | grep -o '"version":"[^"]*"' | cut -d'"' -f4
    elif command -v wget &> /dev/null; then
        wget -qO- https://app.chainloop.dev/api/info | grep -o '"version":"[^"]*"' | cut -d'"' -f4
    else
        echo "v0.256.0"  # fallback version
    fi
}

test_section "Policy Validation"
test_policy_lint "policy.yaml"

test_policy_eval "No Allowed Hostnames - Should Fail the evaluation" "failed_eval" \
    --kind EVIDENCE \
    --material testdata/empty.json

test_policy_eval "With Wrong Allowed Hostname - Should Fail" "failed_eval" \
    --kind EVIDENCE \
    --material testdata/empty.json \
    --allowed-hostnames example.com

test_policy_eval "With Correct Allowed Hostname  should run evaluation but fail because of version mismatch" "fail" \
    --kind EVIDENCE \
    --material testdata/empty.json \
    --allowed-hostnames app.chainloop.dev

echo "Fetching current platform version..."
CURRENT_VERSION=$(get_platform_version)
echo "Current platform version: $CURRENT_VERSION"
echo ""

test_policy_eval "Custom Expected Version (matching current platform)" "pass" \
    --kind EVIDENCE \
    --material testdata/empty.json \
    --allowed-hostnames app.chainloop.dev \
    --input expected_version=$CURRENT_VERSION

# Print test summary and exit
test_summary