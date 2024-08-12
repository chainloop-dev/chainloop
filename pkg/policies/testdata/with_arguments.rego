package main

import rego.v1

deny contains msg if {
	not valid_developer

	msg := sprintf("Invalid developer found: %s", [input.args.email])
}

valid_developer if {
	some subject in input.subject
	subject.annotations["author.email"] == input.args.email
}
