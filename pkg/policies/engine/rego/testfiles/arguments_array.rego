package main

import rego.v1

deny contains msg if {
	"bar" in input.args.foo

	msg := "foo has bar"
}
