package main

import rego.v1

################################
# Common section do NOT change #
################################

result := {
    "skipped": skipped,
    "violations": violations,
    "findings": findings,
    "skip_reason": skip_reason,
}

default skip_reason := ""

skip_reason := m if {
    not valid_input
    m := "invalid input"
}

default skipped := true

skipped := false if valid_input

########################################
# EO Common section, custom code below #
########################################

valid_input := true

# Legacy string violations — should be ignored when findings is present
violations contains msg if {
    input.has_string_violation
    msg := "simple string violation"
}

# Structured findings take precedence over violations
findings contains v if {
    some vuln in input.vulnerabilities
    v := {
        "message": sprintf("Found vulnerability %s", [vuln.id]),
        "external_id": vuln.id,
        "severity": vuln.severity,
    }
}
