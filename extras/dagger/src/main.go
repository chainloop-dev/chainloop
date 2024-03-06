// Chainloop is an open source project that allows you to collect, attest, and distribute pieces of evidence from your Software Supply Chain.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const chainloopVersion = "v0.75.2"

type Chainloop struct {
	Token *Secret
}

func New(token *Secret) *Chainloop {
	return &Chainloop{token}
}

// Start the attestation crafting process
func (m *Chainloop) AttestationInit(
	ctx context.Context,
	// Workflow Contract revision, default is the latest
	// +optional
	contractRevision string,
	// Path to the git repository to be attested
	// +optional
	repository *Directory,
) (string, error) {
	// Append the contract revision to the args if provided
	args := []string{"attestation", "init", "--remote-state", "-o", "json"}
	if contractRevision != "" {
		args = append(args, "--contract-revision", contractRevision)
	}

	// Mount the repository path if provided
	c := m.cliImage()
	if repository != nil {
		c = c.WithDirectory(".", repository)
	}

	info, err := c.WithExec(args).Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("running attestation init: %w", err)
	}

	resp := struct {
		AttestationID string `json:"AttestationID"`
	}{}

	if err := json.Unmarshal([]byte(info), &resp); err != nil {
		return "", fmt.Errorf("unmarshalling attestation init response: %w", err)
	}

	return resp.AttestationID, nil
}

// Check the status of the current attestation
func (m *Chainloop) AttestationStatus(ctx context.Context, attestationID string) (string, error) {
	return m.cliImage().WithExec([]string{
		"attestation", "status",
		"--remote-state",
		"--attestation-id", attestationID,
		"--full",
	}).Stdout(ctx)
}

// Add a piece of evidence/material to the current attestation
// The material value can be provided either in the form of a file or as a raw string
// The file type is required for materials of kind ARTIFACT that are uploaded to the CAS
func (m *Chainloop) AttestationAdd(
	ctx context.Context,
	attestationID string,
	// material name
	name string,
	// path to the file to be added
	// +optional
	path *File,
	// raw value to be added
	// +optional
	value string,
	// Container Registry Credentials for Container image-based materials
	// i.e docker.io, ghcr.io, etc
	// +optional
	registry string,
	// +optional
	registryUsername string,
	// +optional
	registryPassword *Secret,
) (string, error) {
	// Validate that either the path or the raw value is provided
	if value != "" && path != nil {
		return "", fmt.Errorf("only one of material path or value can be provided")
	}

	c := m.cliImage()
	// These OCI credentials are used to resolve materials of type CONTAINER_IMAGE
	if registry != "" {
		c = c.WithEnvVariable("CHAINLOOP_REGISTRY_SERVER", registry).
			WithEnvVariable("CHAINLOOP_REGISTRY_USERNAME", registryUsername).
			WithSecretVariable("CHAINLOOP_REGISTRY_PASSWORD", registryPassword)
	}

	// if the value is provided in a file we need to upload it to the container
	if path != nil {
		fileName, err := path.Name(ctx)
		if err != nil {
			return "", fmt.Errorf("getting file name: %w", err)
		}

		value = fmt.Sprintf("/tmp/%s", fileName)
		c = c.WithFile(value, path)
	}

	return c.WithExec([]string{
		"attestation", "add",
		"--remote-state",
		"--attestation-id", attestationID,
		"--name", name,
		"--value", value,
	}).Stderr(ctx)
}

// Generate, sign and push the attestation to the control plane
func (m *Chainloop) AttestationPush(ctx context.Context, attestationID string, signingKey, passphrase *Secret) (string, error) {
	return m.cliImage().
		WithMountedSecret("/tmp/key.pem", signingKey).
		WithSecretVariable("CHAINLOOP_SIGNING_PASSWORD", passphrase).
		WithExec([]string{
			"attestation", "push",
			"--remote-state",
			"--attestation-id", attestationID,
			"--key", "/tmp/key.pem",
		}).Stdout(ctx)
}

// Mark current attestation process as canceled or failed. --trigger  "failure" | "cancellation" (default: "failure")
func (m *Chainloop) AttestationReset(ctx context.Context,
	attestationID string,
	// +optional
	trigger string,
	// +optional
	reason string) (string, error) {
	args := []string{
		"attestation", "reset",
		"--remote-state",
		"--attestation-id", attestationID,
	}

	if reason != "" {
		args = append(args, "--reason", reason)
	}

	if trigger != "" {
		args = append(args, "--trigger", trigger)
	}

	return m.cliImage().WithExec(args).Stdout(ctx)
}

func (m *Chainloop) cliImage() *Container {
	return dag.Container().
		From(fmt.Sprintf("ghcr.io/chainloop-dev/chainloop/cli:%s", chainloopVersion)).
		WithSecretVariable("CHAINLOOP_ROBOT_ACCOUNT", m.Token).
		WithEnvVariable("CACHEBUSTER", time.Now().String()).
		WithEnvVariable("CHAINLOOP_DAGGER_CLIENT", chainloopVersion)
}
