package main

import rego.v1

# Verifies there is a VEX material, even if not enforced by contract

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
    not has_vex
    msg := "missing VEX material"
}

# Collect all material types
kinds contains kind if {
    some material in input.predicate.materials
    kind := material.annotations["chainloop.material.type"]
}

has_vex if {
    "CSAF_VEX" in kinds
}

has_vex if {
    "OPENVEX" in kinds
}
