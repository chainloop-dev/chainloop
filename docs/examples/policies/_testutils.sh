#!/bin/bash
#
# Chainloop Policy Test Utilities
# 
# This file contains shared test framework functions for policy testing.
# It's designed to be shared across all policy directories.
# 
# Usage: source ../_testutils.sh
#

set -e

# Unalias grep to use standard grep instead of rg
unalias grep 2>/dev/null || true

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Find chainloop binary - prefer PATH
find_chainloop_binary() {
    if command -v chainloop &> /dev/null; then
        echo "chainloop"
    elif [ -f "../../../../app/cli/bin/chainloop" ]; then
        echo "../../../../app/cli/bin/chainloop"
    elif [ -f "../../../../bin/chainloop" ]; then
        echo "../../../../bin/chainloop"
    else
        echo -e "${RED}Error: chainloop binary not found${NC}" >&2
        echo "Please build the CLI first or ensure it's in your PATH" >&2
        exit 1
    fi
}

# Initialize test framework
init_tests() {
    CHAINLOOP_BIN=$(find_chainloop_binary)
    echo -e "${BLUE}Using Chainloop binary: ${CHAINLOOP_BIN}${NC}"
    echo ""
    
    TESTS_PASSED=0
    TESTS_FAILED=0
}

# Check if policy was actually executed (not skipped or ignored)
is_policy_executed() {
    local output="$1"
    
    # Check if policy was skipped
    if echo "$output" | command grep -q '"skipped": *true'; then
        return 1  # false - policy was skipped
    fi
    
    # Check if policy was ignored
    if echo "$output" | command grep -q '"ignored": *true'; then
        return 1  # false - policy was ignored
    fi
    
    return 0  # true - policy was executed
}

# Check if violations exist in policy output
has_violations() {
    local output="$1"
    
    # Check if violations array is non-empty (not "violations": [])
    if echo "$output" | command grep -q '"violations": *\[\]'; then
        return 1  # false - empty violations array
    elif echo "$output" | command grep -q '"violations": *\['; then
        return 0  # true - has violations
    else
        return 1  # false - no violations field found
    fi
}

# Test policy linting
test_policy_lint() {
    local policy_file="$1"
    local test_name="${2:-Policy Lint Check}"
    
    echo -e "${YELLOW}Testing: ${test_name}${NC}"
    echo "Command: $CHAINLOOP_BIN policy develop lint --policy $policy_file"
    
    if output=$($CHAINLOOP_BIN policy develop lint --policy "$policy_file" 2>&1); then
        echo -e "${GREEN}✓ PASSED${NC}"
        ((TESTS_PASSED++)) || true
    else
        echo -e "${RED}✗ FAILED${NC}"
        echo "Output: $output"
        ((TESTS_FAILED++)) || true
    fi
    echo ""
}

# Test policy evaluation
test_policy_eval() {
    local test_name="$1"
    local expected_result="$2"  # "pass" or "fail"
    shift 2
    local args="$@"
    
    # Extract --kind parameter from args
    local material_kind="EVIDENCE"  # Default fallback
    local remaining_args=""
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --kind)
                material_kind="$2"
                shift 2
                ;;
            *)
                remaining_args="$remaining_args $1"
                shift
                ;;
        esac
    done
    
    echo -e "${YELLOW}Testing: ${test_name}${NC}"
    echo "Command: $CHAINLOOP_BIN policy develop eval --policy policy.yaml --kind $material_kind $remaining_args"
    
    # Execute the command
    if output=$($CHAINLOOP_BIN policy develop eval --policy policy.yaml --kind "$material_kind" $remaining_args 2>&1); then
        exit_code=0
    else
        exit_code=$?
    fi
    
    # Determine actual result
    if [ $exit_code -eq 0 ]; then
        # First check if policy was actually executed
        if ! is_policy_executed "$output"; then
            actual_result="fail"  # Policy was skipped or ignored
            if echo "$output" | command grep -q '"skipped": *true'; then
                skip_reason="Policy was skipped"
            elif echo "$output" | command grep -q '"ignored": *true'; then
                skip_reason="Policy was ignored (material type mismatch)"
            else
                skip_reason="Policy was not executed"
            fi
        elif has_violations "$output"; then
            actual_result="fail"  # Has violations = test should fail
        else
            actual_result="pass"  # No violations = test should pass
        fi
    else
        actual_result="failed_eval"  # Command failed to execute
    fi
    
    # Compare with expected result
    if [ "$actual_result" = "$expected_result" ]; then
        case "$expected_result" in
            "pass")
                echo -e "${GREEN}✓ PASSED${NC}"
                ;;
            "fail")
                echo -e "${GREEN}✓ FAILED (as expected)${NC}"
                ;;
            "failed_eval")
                echo -e "${GREEN}✓ EVAL FAILED (as expected)${NC}"
                ;;
        esac
        ((TESTS_PASSED++)) || true
    else
        case "$expected_result" in
            "pass")
                echo -e "${RED}✗ FAILED (expected to pass but $actual_result)${NC}"
                ;;
            "fail")
                echo -e "${RED}✗ PASSED (expected to fail but $actual_result)${NC}"
                ;;
            "failed_eval")
                echo -e "${RED}✗ PASSED (expected eval failure but $actual_result)${NC}"
                ;;
        esac
        
        # Show skip reason if policy wasn't executed
        if [ -n "${skip_reason:-}" ]; then
            echo "Reason: $skip_reason"
        fi
        echo "Output: $output"
        ((TESTS_FAILED++)) || true
    fi
    echo ""
}

# Print test summary and exit with appropriate code
test_summary() {
    echo -e "${BLUE}=== Test Results Summary ===${NC}"
    echo ""
    TOTAL_TESTS=$((TESTS_PASSED + TESTS_FAILED))
    echo -e "Total Tests: ${TOTAL_TESTS}"
    echo -e "${GREEN}Passed: ${TESTS_PASSED}${NC}"
    echo -e "${RED}Failed: ${TESTS_FAILED}${NC}"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}🎉 All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}❌ Some tests failed. Please check the output above.${NC}"
        exit 1
    fi
}

# Verify required files exist
verify_files() {
    local files=("$@")
    for file in "${files[@]}"; do
        if [ ! -f "$file" ]; then
            echo -e "${RED}Error: $file not found${NC}" >&2
            exit 1
        fi
    done
}

# Print a test section header
test_section() {
    local section_name="$1"
    echo -e "${BLUE}=== ${section_name} ===${NC}"
    echo ""
}