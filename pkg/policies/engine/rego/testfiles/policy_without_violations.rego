package main

import rego.v1

# This policy intentionally has no violations or result rule to test error handling

is_approved if {
	input.kind == "CONTAINER_IMAGE"
	input.references[i].metadata.name == "chainloop-platform-qa-approval"
	input.references[i].annotations.approval == "true"
}

is_released if {
	input.kind == "CONTAINER_IMAGE"
	input.references[i].metadata.name == "chainloop-platform-release-production"
}
