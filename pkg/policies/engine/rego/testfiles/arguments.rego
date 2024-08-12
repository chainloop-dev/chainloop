package main

import rego.v1

deny contains msg if {
	input.args.foo == "bar"

	msg := "foo is bar"
}
