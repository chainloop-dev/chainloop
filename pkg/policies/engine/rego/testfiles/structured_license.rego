package main

import rego.v1

################################
# Common section do NOT change #
################################

result := {
    "skipped": skipped,
    "findings": findings,
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

valid_input := true

# Returns structured license finding objects
findings contains v if {
    some comp in input.components
    some license in comp.licenses
    license in input.banned_licenses
    v := {
        "message": sprintf("Banned license %s found in component %s", [license, comp.name]),
        "component_name": comp.name,
        "package_purl": comp.purl,
        "license_id": license,
        "component_version": comp.version,
    }
}
