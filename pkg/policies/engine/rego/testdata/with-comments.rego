package main

import rego.v1

# This is a custom policy
# It checks for violations

violations contains msg if {
    # Check something
    msg := "test violation"
}

# Helper function
has_field if {
    input.field
}
