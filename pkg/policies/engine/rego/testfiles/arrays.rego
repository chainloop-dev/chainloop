package main

import rego.v1

result := {
  "violations": [],
  "skipped": true,
  "skip_reason": sprintf("%d", [count(input.elements)])
}
