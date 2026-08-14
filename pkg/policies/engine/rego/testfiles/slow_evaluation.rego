package main

import rego.v1

# Deliberately expensive computation used to verify that rego evaluation is
# bounded by the engine execution timeout. Materializing the cross product
# takes several seconds, so any short timeout must interrupt it.
expensive := count([x |
	some i in numbers.range(1, 1500)
	some j in numbers.range(1, 1500)
	x := i * j
])

result := {
	"skipped": false,
	"skip_reason": "",
	"violations": violations,
}

violations contains msg if {
	expensive > 0
	msg := "expensive computation completed"
}

matches_evaluation if {
	expensive > 0
}
