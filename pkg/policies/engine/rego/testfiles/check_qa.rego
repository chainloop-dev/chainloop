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
	not is_released
	msg := "Container image is not released"
}

violations contains msg if {
	valid_input
	not is_approved
	msg := "Container image is not approved"
}

is_approved if {
	input.kind == "CONTAINER_IMAGE"

	input.references[i].metadata.name == "chainloop-platform-qa-approval"
	input.references[i].annotations.approval == "true"
}

is_released if {
	input.kind == "CONTAINER_IMAGE"

	input.references[i].metadata.name == "chainloop-platform-release-production"
}
