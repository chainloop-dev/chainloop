package test

import rego.v1

result := {
	"violations": violations,
	"skipped": false,
}

violations contains msg if {
	response := chainloop.hello("world")
	response.message != "Hello, world!"
	msg := sprintf("unexpected message! %s", [response.message])
}
