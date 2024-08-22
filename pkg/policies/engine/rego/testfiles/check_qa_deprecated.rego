package main

deny[msg] {
	not is_released

	msg:= "Container image is not released"
}

deny[msg] {
	not is_approved

	msg:= "Container image is not approved"
}

is_approved {
	input.kind == "CONTAINER_IMAGE"

	input.references[i].metadata.name == "chainloop-platform-qa-approval"
	input.references[i].annotations.approval == "true"
}

is_released {
	input.kind == "CONTAINER_IMAGE"

	input.references[i].metadata.name == "chainloop-platform-release-production"
}
