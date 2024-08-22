package main

import rego.v1

violations contains msg if {
	input.args.foo == "bar"

	msg := "foo is bar"
}
