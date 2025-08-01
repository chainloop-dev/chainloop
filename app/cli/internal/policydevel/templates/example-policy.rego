package main

import rego.v1

################################
# Common section do NOT change #
################################

result := {
	"skipped": skipped,
	"violations": violations,
	"skip_reason": skip_reason,
	"ignore": ignore,
}

default skip_reason := ""

skip_reason := m if {
	not valid_input
	m := "invalid input"
}

default skipped := true

skipped := false if valid_input

default ignore := false

########################################
# EO Common section, custom code below #
########################################
# Validates if the input is valid and can be understood by this policy
valid_input := true

# insert code here

# If the input is valid, check for any policy violation here
default violations := []

# violations contains msg if {
#	valid_input
# insert code here
# }