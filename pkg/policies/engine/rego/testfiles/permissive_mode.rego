package main

import rego.v1

violations contains msg if {
    opa.runtime()
    trace()
    rego.parse_module("", "")

    msg := ""
}
