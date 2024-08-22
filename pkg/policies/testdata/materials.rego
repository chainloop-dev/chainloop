package main

import future.keywords.in
import future.keywords.contains

# Verifies there is a VEX material, even if not enforced by contract

violations[msg] {
    not has_vex

    msg := "missing VEX material"
}

# Collect all material types
kinds contains kind {
    some material in input.predicate.materials
    kind := material.annotations["chainloop.material.type"]
}

has_vex {
    "CSAF_VEX" in kinds
}

has_vex {
    "OPENVEX" in kinds
}
