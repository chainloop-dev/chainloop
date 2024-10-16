package main

import rego.v1

violations contains msg if {
    kev := http.send({"method": "GET", "url": "https://www.chainloop.dev", "cache": true}).body

    msg := ""
}