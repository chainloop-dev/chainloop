package main

import rego.v1

violations contains msg if {
	not has_commit
	msg := "missing commit in statement"
}

has_commit if {
	some sub in input.subject
	sub.name == "git.head"
}
