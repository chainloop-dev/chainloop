package main

import rego.v1

result := {"test": true}
skipped := false
valid_input := true

violations contains msg if {
    msg := "test"
}
