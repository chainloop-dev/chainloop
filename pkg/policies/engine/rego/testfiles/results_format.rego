package main

import rego.v1

result := {
	"passed": passed,
	"violations": violations,
	"message": message,
}

default message := ""

message := m if {
	not valid_input
	m := "invalid input"
}

message := m if {
	valid_input
	count(violations) > 0
	m := "there were violations"
}

default passed := false

passed if {
	valid_input
	count(violations) == 0
}

violations contains msg if {
	valid_input
  input.specVersion != "1.5"
  msg := sprintf("wrong CycloneDX version. Expected 1.5, but it was %s", [input.specVersion])
}

valid_input if {
	input.specVersion
}
