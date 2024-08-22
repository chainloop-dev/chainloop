package main

import rego.v1

violations contains msg if {
	not valid_developer

	msg := "Invalid developer"
}

valid_developer if {
	some subject in input.subject
	subject.annotations["author.email"] == input.args.email
}


valid_developer if {
	some subject in input.subject
	subject.annotations["author.email"] in input.args.email_array
}
