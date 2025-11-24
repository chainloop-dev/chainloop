package main

import rego.v1

default skipped := true

skipped := false if valid_input

violations contains msg if {
    msg := "test violation"
}
