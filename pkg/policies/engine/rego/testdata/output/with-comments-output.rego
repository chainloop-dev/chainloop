package main

import rego.v1

result := {
    "skipped": skipped,
    "violations": violations,
    "skip_reason": skip_reason,
}

default skip_reason := ""

skip_reason := m if {
    not valid_input
    m := "the file content is not recognized"
}

default skipped := true

skipped := false if valid_input

default valid_input := true

# This is a custom policy
# It checks for violations

violations contains msg if {
    # Check something
    msg := "test violation"
}

# Helper function
has_field if {
    input.field
}
