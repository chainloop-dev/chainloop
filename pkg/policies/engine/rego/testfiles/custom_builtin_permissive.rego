package test

import rego.v1

result := {
	"violations": violations,
	"skipped": false,
}

violations contains msg if {
	# Use the custom HTTP built-in
	response := chainloop.http_with_auth("https://api.example.com/check", {"Authorization": "Bearer token123"})
	response.status != 200
	msg := "API check failed"
}

violations contains msg if {
	response := chainloop.http_with_auth("https://api.example.com/check", {"Authorization": "Bearer token123"})
	response.body.allowed != true
	msg := "API returned not allowed"
}
