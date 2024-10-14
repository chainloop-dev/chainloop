package main

import rego.v1

# Common section (1)

# Main rule (2)
result := {
    "passed": passed,
    "violations": violations,
    "message": message,
}

default message := ""

message := m if {
    not valid_input
    m := "invalid input"
}

default passed := false

passed if {
    valid_input
    count(violations) == 0
}

# End of common section

# Validates if the input is valid and can be understood by this policy (3)
valid_input if {
    # insert code here
}

# If the input is valid, check for any policy violation here (4)
violations contains msg if {
    valid_input
    # insert code here
}
