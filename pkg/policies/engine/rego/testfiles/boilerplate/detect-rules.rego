package main

import rego.v1

result := {"test": true}
default skipped := false
default valid_input := true

violations contains msg if {
    msg := "test"
}
