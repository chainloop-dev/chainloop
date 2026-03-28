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

# Returns structured SAST violation objects
violations contains v if {
    some finding in input.findings
    v := {
        "message": sprintf("SAST finding %s in %s", [finding.rule_id, finding.location]),
        "rule_id": finding.rule_id,
        "severity": finding.severity,
        "location": finding.location,
        "line_number": finding.line_number,
        "code_snippet": finding.code_snippet,
    }
}
