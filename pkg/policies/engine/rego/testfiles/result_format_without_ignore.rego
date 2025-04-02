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
	m := "invalid input"
}

default skipped := true

skipped := false if valid_input

violations contains msg if {
	valid_input
	input.specVersion != "1.5"
	msg := sprintf("wrong CycloneDX version. Expected 1.5, but it was %s", [input.specVersion])
}

valid_input if {
	input.specVersion
}
