package main

import rego.v1
import data.lib.helpers
import future.keywords

violations contains msg if {
    msg := "test"
}
