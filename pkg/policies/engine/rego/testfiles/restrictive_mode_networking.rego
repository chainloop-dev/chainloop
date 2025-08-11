package main

import rego.v1

violations contains msg if {
	http.send({"method": "GET", "url": "https://github.com"})

	msg := ""
}
