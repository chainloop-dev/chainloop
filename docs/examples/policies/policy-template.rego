package main

import rego.v1

# (1)
################################
# Common section do NOT change #
################################

# (2)
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

# Validates if the input is valid and can be understood by this policy (3)
valid_input if {
    # insert code here
}

# If the input is valid, check for any policy violation here (4)
violations contains msg if {
    valid_input
    # insert code here
}
