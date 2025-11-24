package main

import rego.v1

valid_input if {
    input.type == "attestation"
}

violations contains msg if {
    not input.subject
    msg := "missing subject"
}
