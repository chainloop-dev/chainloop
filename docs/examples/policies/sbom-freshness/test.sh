#!/bin/bash

# SBOM Freshness Policy Tests
# This file contains policy-specific test cases

source ../_testutils.sh

# Initialize test framework
init_tests

# Verify required files exist
verify_files "policy.yaml" "testdata/sbom-fresh.json" "testdata/sbom-old.json" "testdata/sbom-missing-timestamp.json"

test_section "Policy Validation"
test_policy_lint "policy.yaml"

test_section "Positive Test Scenarios (Should Pass)"

test_policy_eval "Fresh SBOM - Default 30 Days" "pass" \
    --kind SBOM_CYCLONEDX_JSON \
    --material testdata/sbom-fresh.json

test_policy_eval "Fresh SBOM - Custom 60 Days Limit" "pass" \
    --kind SBOM_CYCLONEDX_JSON \
    --material testdata/sbom-fresh.json \
    --input freshness_days=60

test_policy_eval "Old SBOM - Custom 500 Days Limit (Very Permissive)" "pass" \
    --kind SBOM_CYCLONEDX_JSON \
    --material testdata/sbom-old.json \
    --input freshness_days=500

test_section "Negative Test Scenarios (Should Fail)"

test_policy_eval "Old SBOM - Default 30 Days" "fail" \
    --kind SBOM_CYCLONEDX_JSON \
    --material testdata/sbom-old.json

test_policy_eval "Old SBOM - Custom 10 Days Limit" "fail" \
    --kind SBOM_CYCLONEDX_JSON \
    --material testdata/sbom-old.json \
    --input freshness_days=10

test_policy_eval "Missing Timestamp - Should Fail" "fail" \
    --kind SBOM_CYCLONEDX_JSON \
    --material testdata/sbom-missing-timestamp.json

test_section "Custom Freshness Limits"

test_policy_eval "Fresh SBOM - Strict 1 Day Limit" "fail" \
    --kind SBOM_CYCLONEDX_JSON \
    --material testdata/sbom-fresh.json \
    --input freshness_days=1

test_policy_eval "Fresh SBOM - Moderate 15 Days Limit" "fail" \
    --kind SBOM_CYCLONEDX_JSON \
    --material testdata/sbom-fresh.json \
    --input freshness_days=15

# Print test summary and exit
test_summary