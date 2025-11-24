package main

import rego.v1

result := {
    "skipped": skipped,
    "violations": violations,
    "skip_reason": skip_reason,
    "ignore": ignore,
}

default skip_reason := ""

skip_reason := m if {
    not valid_input
    m := "the file content is not recognized"
}

default valid_input := true

default skipped := true

skipped := false if valid_input

default ignore := false

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
