package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	// https://github.com/chainloop-dev/chainloop/releases/tag/v0.60.0
	// providing a sha triggers a no-sec hardcoded credentials false positive
	//nolint:gosec
	clImage = "ghcr.io/chainloop-dev/chainloop/cli@sha256:4e0bc402f71f4877a1ae8d6df5eb4e666a0efa0e7d43ab4f97f21c0e46ae0a59"
)

type Chainloop struct {
	Token *Secret
}

func New(token *Secret) *Chainloop {
	return &Chainloop{token}
}

// Start the attestation crafting process
func (m *Chainloop) AttestationInit(ctx context.Context) (string, error) {
	info, err := m.cliImage().WithExec([]string{"attestation", "init", "--remote-state", "-o", "json"}).Stdout(ctx)
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
	// material name
	name string,
	// path to the file to be added
	// +optional
	path *File,
	// raw value to be added
	// +optional
	value string,
	attestationID string) (string, error) {

	if value != "" && path != nil {
		return "", fmt.Errorf("only one of material path or value can be provided")
	}

	c := m.cliImage()
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
		From(clImage).
		WithSecretVariable("CHAINLOOP_ROBOT_ACCOUNT", m.Token).
		WithEnvVariable("CACHEBUSTER", time.Now().String())
}
