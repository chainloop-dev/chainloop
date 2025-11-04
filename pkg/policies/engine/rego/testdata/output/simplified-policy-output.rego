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

violations contains msg if {
	not has_commit
	msg := "missing commit in statement"
}

has_commit if {
	some sub in input.subject
	sub.name == "git.head"
}
