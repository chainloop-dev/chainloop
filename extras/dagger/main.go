// Chainloop is an open source project that allows you to collect, attest, and distribute pieces of evidence from your Software Supply Chain.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"main/internal/dagger"
	"time"
)

const (
	chainloopVersion = "v0.160.0"
)

var execOpts = dagger.ContainerWithExecOpts{
	UseEntrypoint: true,
}

type Chainloop struct {
	// +private
	Instance InstanceInfo
}

// A Chainloop attestation
// https://docs.chainloop.dev/how-does-it-work/#contract-based-attestation
type Attestation struct {
	AttestationID string

	repository *dagger.Directory

	// +private
	Token *dagger.Secret

	// +private
	RegistryAuth RegistryAuth
	Client       *Chainloop
}

// Configuration for a container registry client
type RegistryAuth struct {
	// Address of the registry
	Address string
	// Username to use when authenticating to the registry
	Username string
	// Password to use when authenticating to the registry
	Password *dagger.Secret
}

// Configuration for a Chainloop instance
type InstanceInfo struct {
	// hostname for the Control Plane API i.e mycontrolplane:443
	ControlplaneAPI string
	// path to a custom CA for the Control Plane API
	ControlplaneCAPath *dagger.File
	// hostname for the cas API i.e myCAS:443
	CASAPI string
	// path to a custom CA for the CAS API
	CASCAPath *dagger.File
	// Password to use when authenticating to the registry
	Insecure bool
}

// Initialize a new attestation
func (m *Chainloop) Init(
	ctx context.Context,
	// Chainloop API token
	token *dagger.Secret,
	// Workflow Contract revision, default is the latest
	// +optional
	contractRevision string,
	// Path to the source repository to be attested
	// +optional
	repository *dagger.Directory,
	// Workflow name to be used for the attestation
	workflowName string,
	// Project name to be used for the attestation
	projectName string,
	// name of an existing contract to attach it to the auto-created workflow
	// +optional
	contractName string,
	// Version of the project to be used for the attestation
	// +optional
	projectVersion string,
	// mark the version as release
	// +optional
	release bool,
) (*Attestation, error) {
	att := &Attestation{
		Token:      token,
		repository: repository,
		Client:     m,
	}
	// Append the contract revision to the args if provided
	args := []string{
		"attestation", "init", "--remote-state", "-o", "json", "--workflow", workflowName, "--project", projectName,
	}

	if contractRevision != "" {
		args = append(args,
			"--contract-revision", contractRevision,
		)
	}

	if contractName != "" {
		args = append(args,
			"--contract", contractName,
		)
	}

	if projectVersion != "" {
		args = append(args,
			"--version", projectVersion,
		)
	}

	if release {
		args = append(args,
			"--release",
		)
	}

	info, err := att.
		Container(0).
		WithExec(args, execOpts).
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

// Resume an attestation from its identifier
func (m *Chainloop) Resume(
	// The attestation ID
	attestationID string,
	// Chainloop API token
	token *dagger.Secret,
) *Attestation {
	return &Attestation{
		AttestationID: attestationID,
		Token:         token,
		Client:        m,
	}
}

// Check the attestation status
func (att *Attestation) Status(ctx context.Context) (string, error) {
	return att.
		Container(0).
		WithExec([]string{
			"attestation", "status",
			"--attestation-id", att.AttestationID,
			"--full",
		}, execOpts).
		Stdout(ctx)
}

// Sync will force the client to send an actual query to the chainloop control plane
// This is specially important to be run right after Init
// for example
//
//	att := chainloop.Init(ctx, token, "main")
//
//	if err := att.Sync(ctx); err != nil {
//		return nil, err
//	}
func (att *Attestation) Sync(_ context.Context) error {
	return nil
}

// Attach credentials for a container registry.
// Chainloop will use them to query the registry for container image pieces of evidences
func (att *Attestation) WithRegistryAuth(
	_ context.Context,
	// Registry address.
	// Example: "index.docker.io"
	address string,
	// Registry username
	username string,
	// Registry password
	password *dagger.Secret,
) *Attestation {
	att.RegistryAuth.Address = address
	att.RegistryAuth.Username = username
	att.RegistryAuth.Password = password
	return att
}

// Configure the Chainloop instance to use
func (m *Chainloop) WithInstance(
	_ context.Context,
	// Example: "api.controlplane.company.com:443"
	controlplaneAPI string,
	// Example: "api.cas.company.com:443"
	casAPI string,
	// Path to custom CA certificate for the CAS API
	// +optional
	casCA *dagger.File,
	// Path to custom CA certificate for the Control Plane API
	// +optional
	controlplaneCA *dagger.File,
	// Whether to skip TLS verification
	// +optional
	insecure bool,
) *Chainloop {
	m.Instance = InstanceInfo{
		ControlplaneAPI:    controlplaneAPI,
		CASAPI:             casAPI,
		Insecure:           insecure,
		CASCAPath:          casCA,
		ControlplaneCAPath: controlplaneCA,
	}

	return m
}

// Add a raw string piece of evidence to the attestation
func (att *Attestation) AddRawEvidence(
	ctx context.Context,
	// Evidence name. Don't pass a name if the material
	// being attested is not part of the contract
	//  Example: "my-blob"
	// +optional
	name string,
	// The contents of the blob
	value string,
) (*Attestation, error) {
	args := []string{
		"attestation", "add",
		"--attestation-id", att.AttestationID,
		"--value", value,
	}

	if name != "" {
		args = append(args,
			"--name", name,
		)
	}

	_, err := att.
		Container(0).
		WithExec(args, execOpts).
		Stdout(ctx)
	return att, err
}

// Add a file type piece of evidence to the attestation
func (att *Attestation) AddFileEvidence(
	ctx context.Context,
	// Evidence name. Don't pass a name if the material
	// being attested is not part of the contract
	//  Example: "my-binary"
	// +optional
	name string,
	// The file to add
	path *dagger.File,
) (*Attestation, error) {
	filename, err := path.Name(ctx)
	if err != nil {
		return att, err
	}

	mountPath := "/tmp/attestation/" + filename

	args := []string{
		"attestation", "add",
		"--attestation-id", att.AttestationID,
		"--value", mountPath,
	}

	if name != "" {
		args = append(args,
			"--name", name,
		)
	}

	_, err = att.
		Container(0).
		// Preserve the filename inside the container
		WithFile(mountPath, path).
		WithExec(args, execOpts).
		Sync(ctx)

	return att, err
}

func (att *Attestation) Debug() *dagger.Container {
	return att.Container(0).Terminal()
}

func cliContainer(ttl int, token *dagger.Secret, instance InstanceInfo) *dagger.Container {
	ctr := dag.Container().
		From(fmt.Sprintf("ghcr.io/chainloop-dev/chainloop/cli:%s", chainloopVersion)).
		WithEntrypoint([]string{"/chainloop"}). // Be explicit to prepare for possible API change
		WithEnvVariable("CHAINLOOP_DAGGER_CLIENT", chainloopVersion).
		WithUser("").                                                                                     // Our images come with pre-defined user set, so we need to reset it
		WithEnvVariable("DAGGER_CACHE_KEY", time.Now().Truncate(time.Duration(ttl)*time.Second).String()) // Cache TTL

	if token != nil {
		ctr = ctr.WithSecretVariable("CHAINLOOP_TOKEN", token)
	}

	if api := instance.ControlplaneAPI; api != "" {
		ctr = ctr.WithEnvVariable("CHAINLOOP_CONTROL_PLANE_API", api)
	}

	if ca := instance.ControlplaneCAPath; ca != nil {
		ctr = ctr.WithFile("/controlplane-ca.pem", ca).WithEnvVariable("CHAINLOOP_CONTROL_PLANE_API_CA", "/controlplane-ca.pem")
	}

	if ca := instance.CASCAPath; ca != nil {
		ctr = ctr.WithFile("/cas-ca.pem", ca).WithEnvVariable("CHAINLOOP_ARTIFACT_CAS_API_CA", "/cas-ca.pem")
	}

	if cas := instance.CASAPI; cas != "" {
		ctr = ctr.WithEnvVariable("CHAINLOOP_ARTIFACT_CAS_API", cas)
	}

	if instance.Insecure {
		ctr = ctr.WithEnvVariable("CHAINLOOP_API_INSECURE", "true")
	}

	// Cache TTL
	ctr = ctr.WithEnvVariable("DAGGER_CACHE_KEY", time.Now().Truncate(time.Duration(ttl)*time.Second).String())

	return ctr
}

// Build an ephemeral container with everything needed to process the attestation
func (att *Attestation) Container(
	// Cache TTL for chainloop commands, in seconds
	//  Defaults to 0: no caching
	// +optional
	// +default=0
	ttl int,
) *dagger.Container {
	ctr := cliContainer(ttl, att.Token, att.Client.Instance)
	if att.repository != nil {
		ctr = ctr.WithDirectory(".", att.repository)
	}

	if addr := att.RegistryAuth.Address; addr != "" {
		ctr = ctr.WithEnvVariable("CHAINLOOP_REGISTRY_SERVER", addr)
	}

	if user := att.RegistryAuth.Username; user != "" {
		ctr = ctr.WithEnvVariable("CHAINLOOP_REGISTRY_USERNAME", user)
	}

	if pw := att.RegistryAuth.Password; pw != nil {
		ctr = ctr.WithSecretVariable("CHAINLOOP_REGISTRY_PASSWORD", pw)
	}

	return ctr
}

// Generate, sign and push the attestation to the chainloop control plane
func (att *Attestation) Push(
	ctx context.Context,
	// The private key to sign the attestation
	// +optional
	key *dagger.Secret,
	// The passphrase to decrypt the private key
	// +optional
	passphrase *dagger.Secret,
	// Whether not fail if the policy check fails
	// +optional
	exceptionBypassPolicyCheck *bool,
) (string, error) {
	container := att.Container(0)
	args := []string{
		"attestation", "push",
		"--attestation-id", att.AttestationID,
	}

	if key != nil {
		container = container.WithMountedSecret("/tmp/key.pem", key)
		args = append(args, "--key", "/tmp/key.pem")
	}
	if passphrase != nil {
		container = container.WithSecretVariable("CHAINLOOP_SIGNING_PASSWORD", passphrase)
	}
	if exceptionBypassPolicyCheck != nil && *exceptionBypassPolicyCheck {
		args = append(args, "--exception-bypass-policy-check")
	}

	return container.WithExec(args, execOpts).Stdout(ctx)
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
		WithExec(args, execOpts).
		Sync(ctx)
	return err
}

/// standalone API calls

// Create a new workflow
func (m *Chainloop) WorkflowCreate(
	ctx context.Context,
	// Chainloop API token
	token *dagger.Secret,
	// Workflow name
	name string,
	// Workflow project
	project string,
	// +optional
	team string,
	// +optional
	description string,
	// name of an existing contract
	// +optional
	contractName string,
	// Set workflow as public so other organizations can see it
	// +optional
	public bool,
	// If the workflow already exists, skip the creation and return success
	// +optional
	skipIfExists bool,
) (string, error) {
	return cliContainer(0, token, m.Instance).
		WithExec([]string{
			"workflow", "create",
			"--name", name,
			"--project", project,
			"--team", team,
			"--description", description,
			"--contract", contractName,
			"--public", fmt.Sprintf("%t", public),
			"--skip-if-exists", fmt.Sprintf("%t", skipIfExists),
		}, execOpts).
		Stdout(ctx)
}
