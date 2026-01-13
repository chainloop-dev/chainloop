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
	chainloopVersion = "v1.70.0"
)

var execOpts = dagger.ContainerWithExecOpts{
	UseEntrypoint: true,
}

type Chainloop struct {
	// +private
	Instance InstanceInfo
}

// A Chainloop attestation
// https://docs.chainloop.dev/concepts/attestations
type Attestation struct {
	AttestationID string
	OrgName       string

	repository *dagger.Directory

	// +private
	Token *dagger.Secret

	// +private
	RegistryAuth RegistryAuth
	Client       *Chainloop

	// +private
	parentCIContext *ParentCIContext
	// +private
	githubEventFile *dagger.File
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

// ParentCIContext holds environment variables from a parent CI system (Github Actions, Gitlab CI)
// to enable PR/MR auto-detection and commit verification when running Chainloop via Dagger inside those CI systems
type ParentCIContext struct {
	// Github Actions PR context
	// Repository name (owner/repo)
	GithubRepository string
	// Run ID for the workflow run
	GithubRunID string
	// Event name (e.g., "pull_request", "pull_request_target")
	GithubEventName string
	// Source branch name
	GithubHeadRef string
	// Target branch name
	GithubBaseRef string
	// Github token for API access and commit verification
	GithubToken *dagger.Secret

	// Gitlab CI MR context
	// CI indicator (always "true" in Gitlab CI)
	GitlabCI string
	// Server URL (e.g., "https://gitlab.com")
	GitlabServerURL string
	// Project path (e.g., "group/project")
	GitlabProjectPath string
	// Job URL
	GitlabJobURL string
	// Pipeline source (should be "merge_request_event" for MRs)
	GitlabPipelineSource string
	// Merge request internal ID
	GitlabMRIID string
	// Merge request title
	GitlabMRTitle string
	// Merge request description
	GitlabMRDescription string
	// Source branch name
	GitlabMRSourceBranch string
	// Target branch name
	GitlabMRTargetBranch string
	// Project URL
	GitlabMRProjectURL string
	// User login
	GitlabUserLogin string
	// Gitlab job token for API access and commit verification
	GitlabJobToken *dagger.Secret
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
	// Github event file for PR detection (when running in Github Actions)
	// +optional
	githubEventFile *dagger.File,
	// Github repository name (owner/repo)
	// +optional
	githubRepository string,
	// Github run ID for the workflow run
	// +optional
	githubRunID string,
	// Github event name (e.g., "pull_request", "pull_request_target")
	// +optional
	githubEventName string,
	// Github source branch name
	// +optional
	githubHeadRef string,
	// Github target branch name
	// +optional
	githubBaseRef string,
	// Github token for API access and commit verification (when running in Github Actions)
	// +optional
	githubToken *dagger.Secret,
	// Gitlab CI indicator (should be "true" when running in Gitlab CI)
	// +optional
	gitlabCI string,
	// Gitlab server URL (e.g., "https://gitlab.com")
	// +optional
	gitlabServerURL string,
	// Gitlab project path (e.g., "group/project")
	// +optional
	gitlabProjectPath string,
	// Gitlab job URL
	// +optional
	gitlabJobURL string,
	// Gitlab pipeline source (should be "merge_request_event" for MRs)
	// +optional
	gitlabPipelineSource string,
	// Gitlab merge request internal ID
	// +optional
	gitlabMRIID string,
	// Gitlab merge request title
	// +optional
	gitlabMRTitle string,
	// Gitlab merge request description
	// +optional
	gitlabMRDescription string,
	// Gitlab source branch name
	// +optional
	gitlabMRSourceBranch string,
	// Gitlab target branch name
	// +optional
	gitlabMRTargetBranch string,
	// Gitlab project URL
	// +optional
	gitlabMRProjectURL string,
	// Gitlab user login
	// +optional
	gitlabUserLogin string,
	// Gitlab job token for API access and commit verification (when running in Gitlab CI)
	// +optional
	gitlabJobToken *dagger.Secret,
) (*Attestation, error) {
	// Construct ParentCIContext from individual parameters
	var parentCIContext *ParentCIContext
	if githubRepository != "" || githubRunID != "" || githubEventName != "" || githubHeadRef != "" || githubBaseRef != "" ||
		gitlabCI != "" || gitlabServerURL != "" || gitlabProjectPath != "" || gitlabJobURL != "" ||
		gitlabPipelineSource != "" || gitlabMRIID != "" || gitlabMRTitle != "" ||
		gitlabMRDescription != "" || gitlabMRSourceBranch != "" || gitlabMRTargetBranch != "" ||
		gitlabMRProjectURL != "" || gitlabUserLogin != "" {
		parentCIContext = &ParentCIContext{
			GithubRepository:     githubRepository,
			GithubRunID:          githubRunID,
			GithubEventName:      githubEventName,
			GithubHeadRef:        githubHeadRef,
			GithubBaseRef:        githubBaseRef,
			GithubToken:          githubToken,
			GitlabCI:             gitlabCI,
			GitlabServerURL:      gitlabServerURL,
			GitlabProjectPath:    gitlabProjectPath,
			GitlabJobURL:         gitlabJobURL,
			GitlabPipelineSource: gitlabPipelineSource,
			GitlabMRIID:          gitlabMRIID,
			GitlabMRTitle:        gitlabMRTitle,
			GitlabMRDescription:  gitlabMRDescription,
			GitlabMRSourceBranch: gitlabMRSourceBranch,
			GitlabMRTargetBranch: gitlabMRTargetBranch,
			GitlabMRProjectURL:   gitlabMRProjectURL,
			GitlabUserLogin:      gitlabUserLogin,
			GitlabJobToken:       gitlabJobToken,
		}
	}

	att := &Attestation{
		Token:           token,
		repository:      repository,
		Client:          m,
		parentCIContext: parentCIContext,
		githubEventFile: githubEventFile,
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
		WorkflowMeta  struct {
			Organization string
		}
	}
	if err := json.Unmarshal([]byte(info), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal attestation init response: %w", err)
	}

	att.AttestationID = resp.AttestationID
	att.OrgName = resp.WorkflowMeta.Organization

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
	// the material type of the evidence https://docs.chainloop.dev/concepts/material-types#material-types
	// if not provided it will either be loaded from the contract or inferred automatically
	// +optional
	kind string,
	// List of annotations to be attached to the evidence for example:
	// "key1=value1,key2=value2"
	// +optional
	annotations []string,
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

	if kind != "" {
		args = append(args,
			"--kind", kind,
		)
	}

	for _, annotation := range annotations {
		args = append(args,
			"--annotation", annotation,
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
	// the material type of the evidence https://docs.chainloop.dev/concepts/material-types#material-types
	// if not provided it will either be loaded from the contract or inferred automatically
	// +optional
	kind string,
	// List of annotations to be attached to the evidence for example:
	// "key1=value1,key2=value2"
	// +optional
	annotations []string,
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

	for _, annotation := range annotations {
		args = append(args,
			"--annotation", annotation,
		)
	}

	if kind != "" {
		args = append(args,
			"--kind", kind,
		)
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

func cliContainer(ttl int, token *dagger.Secret, instance InstanceInfo, parentCI *ParentCIContext, githubEventFile *dagger.File) *dagger.Container {
	ctr := dag.Container().
		From(fmt.Sprintf("ghcr.io/chainloop-dev/chainloop/cli:%s", chainloopVersion)).
		WithEntrypoint([]string{"/chainloop"}). // Be explicit to prepare for possible API change
		WithEnvVariable("CHAINLOOP_DAGGER_CLIENT", chainloopVersion).
		WithUser("").                                                                                     // Our images come with pre-defined user set, so we need to reset it
		WithEnvVariable("DAGGER_CACHE_KEY", time.Now().Truncate(time.Duration(ttl)*time.Second).String()) // Cache TTL

	// Inject parent CI context if provided
	if parentCI != nil {
		// Github Actions context
		if parentCI.GithubRepository != "" {
			ctr = ctr.WithEnvVariable("GITHUB_REPOSITORY", parentCI.GithubRepository)
		}
		if parentCI.GithubRunID != "" {
			ctr = ctr.WithEnvVariable("GITHUB_RUN_ID", parentCI.GithubRunID)
		}
		if parentCI.GithubEventName != "" {
			ctr = ctr.WithEnvVariable("GITHUB_EVENT_NAME", parentCI.GithubEventName)
		}
		if parentCI.GithubHeadRef != "" {
			ctr = ctr.WithEnvVariable("GITHUB_HEAD_REF", parentCI.GithubHeadRef)
		}
		if parentCI.GithubBaseRef != "" {
			ctr = ctr.WithEnvVariable("GITHUB_BASE_REF", parentCI.GithubBaseRef)
		}
		if parentCI.GithubToken != nil {
			ctr = ctr.WithSecretVariable("GITHUB_TOKEN", parentCI.GithubToken)
		}

		// Handle Github event file (passed as separate parameter for CLI convenience)
		if githubEventFile != nil {
			ctr = ctr.WithFile("/tmp/github_event.json", githubEventFile).
				WithEnvVariable("GITHUB_EVENT_PATH", "/tmp/github_event.json")
		}

		// Gitlab CI context
		if parentCI.GitlabCI != "" {
			ctr = ctr.WithEnvVariable("GITLAB_CI", parentCI.GitlabCI)
		}
		if parentCI.GitlabServerURL != "" {
			ctr = ctr.WithEnvVariable("CI_SERVER_URL", parentCI.GitlabServerURL)
		}
		if parentCI.GitlabProjectPath != "" {
			ctr = ctr.WithEnvVariable("CI_PROJECT_PATH", parentCI.GitlabProjectPath)
		}
		if parentCI.GitlabJobURL != "" {
			ctr = ctr.WithEnvVariable("CI_JOB_URL", parentCI.GitlabJobURL)
		}
		if parentCI.GitlabPipelineSource != "" {
			ctr = ctr.WithEnvVariable("CI_PIPELINE_SOURCE", parentCI.GitlabPipelineSource)
		}
		if parentCI.GitlabMRIID != "" {
			ctr = ctr.WithEnvVariable("CI_MERGE_REQUEST_IID", parentCI.GitlabMRIID)
		}
		if parentCI.GitlabMRTitle != "" {
			ctr = ctr.WithEnvVariable("CI_MERGE_REQUEST_TITLE", parentCI.GitlabMRTitle)
		}
		if parentCI.GitlabMRDescription != "" {
			ctr = ctr.WithEnvVariable("CI_MERGE_REQUEST_DESCRIPTION", parentCI.GitlabMRDescription)
		}
		if parentCI.GitlabMRSourceBranch != "" {
			ctr = ctr.WithEnvVariable("CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", parentCI.GitlabMRSourceBranch)
		}
		if parentCI.GitlabMRTargetBranch != "" {
			ctr = ctr.WithEnvVariable("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", parentCI.GitlabMRTargetBranch)
		}
		if parentCI.GitlabMRProjectURL != "" {
			ctr = ctr.WithEnvVariable("CI_MERGE_REQUEST_PROJECT_URL", parentCI.GitlabMRProjectURL)
		}
		if parentCI.GitlabUserLogin != "" {
			ctr = ctr.WithEnvVariable("GITLAB_USER_LOGIN", parentCI.GitlabUserLogin)
		}
		if parentCI.GitlabJobToken != nil {
			ctr = ctr.WithSecretVariable("CI_JOB_TOKEN", parentCI.GitlabJobToken)
		}
	}

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
	ctr := cliContainer(ttl, att.Token, att.Client.Instance, att.parentCIContext, att.githubEventFile)
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

type OutputFormat string

const (
	OutputFormatTable OutputFormat = "table"
	OutputFormatJSON  OutputFormat = "json"
)

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
	// Output format
	// +default="table"
	format OutputFormat,
	// List of annotations to be attached to the attestation for example:
	// "key1=value1,key2=value2"
	// +optional
	annotations []string,
) (string, error) {
	container := att.Container(0)
	args := []string{
		"attestation", "push",
		"--attestation-id", att.AttestationID,
		"--output", string(format),
	}

	for _, annotation := range annotations {
		args = append(args, "--annotation", annotation)
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
	return cliContainer(0, token, m.Instance, nil, nil).
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
