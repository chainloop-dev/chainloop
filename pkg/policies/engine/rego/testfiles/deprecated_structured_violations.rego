package main

import rego.v1

################################
# Common section do NOT change #
################################

result := {
    "skipped": skipped,
    "violations": violations,
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

# Deprecated: structured objects in violations.
# This format is supported temporarily for backward compatibility.
# Policies should migrate to use the "findings" field instead.
violations contains v if {
    some vuln in input.vulnerabilities
    v := {
        "message": sprintf("Found vulnerability %s (%s)", [vuln.id, vuln.severity]),
        "external_id": vuln.id,
        "severity": vuln.severity,
        "package_purl": vuln.purl,
    }
}
