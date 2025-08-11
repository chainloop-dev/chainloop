#!/bin/bash

# JSON Field Validator Policy Tests
# This file contains policy-specific test cases

source ../_testutils.sh

# Initialize test framework
init_tests

# Verify required files exist
verify_files "policy.yaml" "testdata/config.json" "testdata/compliance-checklist.json"

test_section "Policy Validation"
test_policy_lint "policy.yaml"

test_section "Positive Test Scenarios (Should Pass)"

test_policy_eval "Application Name - Correct Value" "pass" \
    --kind EVIDENCE \
    --material testdata/config.json \
    --input required_field=application.name \
    --input expected_value=web-service

test_policy_eval "Application Environment - Correct Value" "pass" \
    --kind EVIDENCE \
    --material testdata/config.json \
    --input required_field=application.environment \
    --input expected_value=production

test_policy_eval "Version Pattern - Semantic Versioning" "pass" \
    --kind EVIDENCE \
    --material testdata/config.json \
    --input required_field=application.version \
    --input field_pattern="^[0-9]+\.[0-9]+\.[0-9]+$"

test_policy_eval "Security Enabled - Boolean True" "pass" \
    --kind EVIDENCE \
    --material testdata/config.json \
    --input required_field=security.enabled \
    --input expected_value=true

test_section "Negative Test Scenarios (Should Fail)"

test_policy_eval "Application Name - Wrong Value" "fail" \
    --kind EVIDENCE \
    --material testdata/config.json \
    --input required_field=application.name \
    --input expected_value=wrong-service

test_policy_eval "Application Environment - Wrong Value" "fail" \
    --kind EVIDENCE \
    --material testdata/config.json \
    --input required_field=application.environment \
    --input expected_value=staging

test_policy_eval "Version Pattern - V-prefixed Pattern" "fail" \
    --kind EVIDENCE \
    --material testdata/config.json \
    --input required_field=application.version \
    --input field_pattern="^v[0-9]+\.[0-9]+\.[0-9]+$"

test_policy_eval "Version Pattern - Major.Minor Only" "fail" \
    --kind EVIDENCE \
    --material testdata/config.json \
    --input required_field=application.version \
    --input field_pattern="^[0-9]+\.[0-9]+$"

test_section "Different JSON Structure Tests"

test_policy_eval "Missing Application Section" "fail" \
    --kind EVIDENCE \
    --material testdata/compliance-checklist.json \
    --input required_field=application.name \
    --input expected_value=test

# Print test summary and exit
test_summary