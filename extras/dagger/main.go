// Chainloop is an open source project that allows you to collect, attest, and distribute pieces of evidence from your Software Supply Chain.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

var (
	chainloopVersion = "v0.75.2"
)

type Chainloop struct {}

// Resume an attestation from its identifier
func (m *Chainloop) Resume(
	// The attestation ID
	attestationID string,
	// Chainloop API token
	token *Secret,
) *Attestation {
	return &Attestation{
		AttestationID: attestationID,
		Token: token,
	}
}

// Initialize a new attestation
func (m *Chainloop) Init(
	ctx context.Context,
	// Workflow Contract revision, default is the latest
	// +optional
	contractRevision string,
	// Path to the source repository to be attested
	// +optional
	source *Directory,
	// Chainloop API token
	// +optional
	token *Secret,
) (*Attestation, error) {
	att := &Attestation{
		Token: token,
		Source: source,
	}
	// Append the contract revision to the args if provided
	args := []string{
		"attestation", "init", "--remote-state", "-o", "json",
	}
	if contractRevision != "" {
		args = append(args,
			"--contract-revision", contractRevision,
		)
	}
	info, err := att.
		Container(0).
		WithExec(args).
		Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("attestation init: %w", err)
	}
	var resp struct {
		AttestationID string
	}
	if err := json.Unmarshal([]byte(info), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal attestation init response: %w", err)
	}
	att.AttestationID = resp.AttestationID
	return att, nil
}

// A Chainloop attestation
// https://docs.chainloop.dev/how-does-it-work/#contract-based-attestation
type Attestation struct {
	AttestationID string `json:"AttestationID"`

	Source *Directory

	// +private
	Token *Secret

	// +private
	Registry RegistryClient
}

// Configuration for a container registry client
type RegistryClient struct {
	// Address of the registry
	Address string
	// Username to use when authenticating to the registry
	Username string
	// Password to use when authenticating to the registry
	Password *Secret
}

// Check the attestation status
func (att *Attestation) Status(ctx context.Context) (string, error) {
	return att.
		Container(0).
		WithExec([]string{
			"attestation", "status",
			"--remote-state",
			"--attestation-id", att.AttestationID,
			"--full",
		}).
		Stdout(ctx)
}

// Attach credentials for a container registry.
// Chainloop will use them to query the registry for container image material.
func (att *Attestation) WithRegistry(
	// Registry address.
	// Example: "index.docker.io"
	address string,
	// Registry username
	username string,
	// Registry password
	password *Secret,
) *Attestation {
	att.Registry.Address = address
	att.Registry.Username = username
	att.Registry.Password = password
	return att
}

// Add a blob of text to the attestation
func (att *Attestation) AddBlob(
	ctx context.Context,
	// Material name.
	//   Example: "my-blob"
	name string,
	// The contents of the blob
	contents string,
) (*Attestation, error) {
	_, err := att.
		Container(0).
		WithExec([]string{
			"attestation", "add",
			"--remote-state",
			"--attestation-id", att.AttestationID,
			"--name", name,
			"--value", contents,
		}).
		Stdout(ctx)
	return att, err
}

// Add a file to the attestation
func (att *Attestation) AddFile(
	ctx context.Context,
	// Material name.
	//  Example: "my-binary"
	name string,
	// The file to add
	file *File,
) (*Attestation, error) {
	filename, err := file.Name(ctx)
	if err != nil {
		return att, err
	}
	mountPath := "/tmp/attestation/" + filename
	_, err = att.
		Container(0).
		// Preserve the filename inside the container
		WithFile(mountPath, file).
		WithExec([]string{
			"attestation", "add",
			"--remote-state",
			"--attestation-id", att.AttestationID,
			"--name", name,
			"--value", mountPath,
		}).
		Sync(ctx)
	return att, err
}

func (att *Attestation) Debug() *Terminal {
	return att.
		Container(0).
		Terminal()
}

// Build an ephemeral container with everything needed to process the attestation
func (att *Attestation) Container(
	// Cache TTL for chainloop commands, in seconds
	//  Defaults to 0: no caching
	// +optional
	// +default=0
	ttl int,
) *Container {
	ctr := dag.
		Container().
		From(fmt.Sprintf("ghcr.io/chainloop-dev/chainloop/cli:%s", chainloopVersion)).
		WithEntrypoint([]string{"/chainloop"}). // Be explicit to prepare for possible API change
		WithEnvVariable("CHAINLOOP_DAGGER_CLIENT", chainloopVersion)
	if att.Token != nil {
		ctr = ctr.WithSecretVariable("CHAINLOOP_ROBOT_ACCOUNT", att.Token)
	}
	if att.Source != nil {
		ctr = ctr.WithMountedDirectory(".", att.Source)
	}
	if addr := att.Registry.Address; addr != "" {
		ctr = ctr.WithEnvVariable("CHAINLOOP_REGISTRY_SERVER", addr)
	}
	if user := att.Registry.Username; user != "" {
		ctr = ctr.WithEnvVariable("CHAINLOOP_REGISTRY_USERNAME", user)
	}
	if pw := att.Registry.Password; pw != nil {
		ctr = ctr.WithSecretVariable("CHAINLOOP_REGISTRY_USERNAME", pw)
	}
	// Cache TTL
	ctr = ctr.WithEnvVariable("DAGGER_CACHE_KEY", time.Now().Truncate(time.Duration(ttl) * time.Second).String())
	return ctr
}

// Generate, sign and push the attestation to the chainloop control plane
func (att *Attestation) Push(ctx context.Context, key, passphrase *Secret) (string, error) {
	return att.
		Container(0).
		WithMountedSecret("/tmp/key.pem", key).
		WithSecretVariable("CHAINLOOP_SIGNING_PASSWORD", passphrase).
		WithExec([]string{
			"attestation", "push",
			"--remote-state",
			"--attestation-id", att.AttestationID,
			"--key", "/tmp/key.pem",
		}).Stdout(ctx)
}

// Mark the attestation as failed
func (att *Attestation) MarkFailed(
	ctx context.Context,
	// The reason for canceling, in human-readable form
	// +optional
	reason string,
) error {
	return att.reset(ctx, "failure", reason)
}

// Mark the attestation as canceled
func (att *Attestation) MarkCanceled(
	ctx context.Context,
	// The reason for canceling, in human-readable form
	// +optional
	reason string,
) error {
	return att.reset(ctx, "cancellation", reason)
}

// Call `chainloop reset` to mark the attestation as either failed or cancelled.
func (att *Attestation) reset(ctx context.Context,
	// +optional
	// The trigger that caused the reset.
	// May be "failure" or "cancellation"
	trigger string,
	// The reason for the reset, in human-readable form
	// +optional
	reason string,
) error {
	args := []string{
		"attestation", "reset",
		"--remote-state",
		"--attestation-id", att.AttestationID,
	}
	if reason != "" {
		args = append(args, "--reason", reason)
	}
	if trigger != "" {
		args = append(args, "--trigger", trigger)
	}
	_, err := att.
		Container(0).
		WithExec(args).
		Sync(ctx)
	return err
}
