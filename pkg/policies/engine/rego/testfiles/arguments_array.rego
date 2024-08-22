package main

import rego.v1

violations contains msg if {
	"bar" in input.args.foo

	msg := "foo has bar"
}
