package main

import rego.v1

################################
# Common section do NOT change #
################################

result := {
    "skipped": skipped,
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

# Returns structured finding objects WITHOUT a "message" field
findings contains v if {
    some vuln in input.vulnerabilities
    v := {
        "external_id": vuln.id,
        "severity": vuln.severity,
    }
}
