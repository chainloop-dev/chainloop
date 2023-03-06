schemaVersion: "v1"
materials: [
	// Binaries
	{type: "ARTIFACT", name: "cli-linux-amd64", output:           true},
	{type: "ARTIFACT", name: "control-plane-linux-amd64", output: true},
	{type: "ARTIFACT", name: "artifact-cas-linux-amd64", output:  true},
	// Container images
	{type: "CONTAINER_IMAGE", name: "control-plane-image", output: true},
	{type: "CONTAINER_IMAGE", name: "artifact-cas-image", output:  true},
	// SBOMS for those container images
	{type: "SBOM_CYCLONEDX_JSON", name: "sbom-control-plane"},
	{type: "SBOM_CYCLONEDX_JSON", name: "sbom-artifact-cas"},
]
runner: type: "GITHUB_ACTION"
