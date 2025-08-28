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

# Validates if the input is valid and can be understood by this policy
valid_input := true

# If the input is valid, check for any policy violation here
violations contains msg if {
	valid_input
	input.args.foo == "bar"
	msg := "foo is bar"
}
