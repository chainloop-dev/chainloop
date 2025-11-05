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

default ignore := false

default skipped := true

skipped := false if valid_input

violations contains msg if {
    msg := "test violation"
}
