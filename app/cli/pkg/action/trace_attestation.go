//
// Copyright 2026 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package action

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	internaltoken "github.com/chainloop-dev/chainloop/app/cli/internal/token"
	"github.com/chainloop-dev/chainloop/pkg/grpcconn"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

// AttestationExecutor calls the attestation actions directly via their Go
// API rather than shelling out, so `chainloop trace` can drive a full
// init/add/push cycle in-process.
type AttestationExecutor struct {
	actionOpts     *ActionsOpts
	cliVersion     string
	localStatePath string
	// ownedConn is non-nil when the executor created its own control-plane
	// connection (e.g. via WithForcedOrganization). The caller must invoke
	// Close to release it.
	ownedConn *grpc.ClientConn
}

type ExecutorOption func(*AttestationExecutor) error

// NewAttestationExecutor creates an executor from the root command's already
// initialized ActionsOpts. Commands using it must NOT set skipActionOptsInit,
// or base will be nil. cliVersion is recorded in the attestation predicate and
// must be the bare version (cmd.Version), not ActionsOpts.CLIVersion, which
// carries an edition suffix.
func NewAttestationExecutor(base *ActionsOpts, cliVersion string, opts ...ExecutorOption) (*AttestationExecutor, error) {
	if base == nil {
		return nil, fmt.Errorf("base action options are required")
	}

	e := &AttestationExecutor{actionOpts: base, cliVersion: cliVersion}
	for _, o := range opts {
		if err := o(e); err != nil {
			_ = e.Close()
			return nil, err
		}
	}

	return e, nil
}

// WithLogger overrides the logger used by attestation operations.
// It clones ActionOpts to avoid mutating global state.
func WithLogger(l zerolog.Logger) ExecutorOption {
	return func(e *AttestationExecutor) error {
		clone := *e.actionOpts
		clone.Logger = l
		e.actionOpts = &clone

		return nil
	}
}

// WithLocalStatePath sets a file path for local attestation state,
// avoiding remote state and conflicts with other concurrent attestations.
func WithLocalStatePath(path string) ExecutorOption {
	return func(e *AttestationExecutor) error {
		e.localStatePath = path

		return nil
	}
}

// WithForcedOrganization replaces the control-plane connection with a fresh one
// that carries the given organization name in every request, overriding the
// CLI's default org. Pass an empty string to keep the default connection.
// The new connection is owned by the executor and closed by Close.
func WithForcedOrganization(orgName string) ExecutorOption {
	return func(e *AttestationExecutor) error {
		if orgName == "" {
			return nil
		}

		if tokenOrg := internaltoken.MismatchedOrganization(e.actionOpts.AuthTokenRaw, orgName); tokenOrg != "" {
			return fmt.Errorf("credentials belong to organization %q but %q is configured: API tokens are bound to their own organization, so mint a token for %q instead", tokenOrg, orgName, orgName)
		}

		conn, err := newControlPlaneConnection(e.actionOpts.AuthTokenRaw, orgName)
		if err != nil {
			return fmt.Errorf("open forced-org control-plane connection: %w", err)
		}

		clone := *e.actionOpts
		clone.CPConnection = conn
		e.actionOpts = &clone
		e.ownedConn = conn

		return nil
	}
}

// Close releases resources owned by the executor, currently any control-plane
// connection created via WithForcedOrganization. Safe to call multiple times.
func (e *AttestationExecutor) Close() error {
	if e.ownedConn == nil {
		return nil
	}

	conn := e.ownedConn
	e.ownedConn = nil

	return conn.Close()
}

// newControlPlaneConnection dials the control plane using the settings from
// viper (same keys as the upstream root command) and pins the given org name.
func newControlPlaneConnection(authToken, orgName string) (*grpc.ClientConn, error) {
	opts := []grpcconn.Option{
		grpcconn.WithInsecure(viper.GetBool("api-insecure")),
		grpcconn.WithOrgName(orgName),
	}

	if caValue := viper.GetString("control-plane.api-ca"); caValue != "" {
		// Mirror upstream behavior: treat as a file path when the file exists,
		// otherwise treat it as raw CA content.
		if _, err := os.Stat(caValue); err == nil {
			opts = append(opts, grpcconn.WithCAFile(caValue))
		} else {
			opts = append(opts, grpcconn.WithCAContent(caValue))
		}
	}

	return grpcconn.New(viper.GetString("control-plane.API"), authToken, opts...)
}

// casURI returns the CAS connection URI from viper config.
func casURI() string { return viper.GetString("artifact-cas.API") }

// casCAPath returns the CAS CA certificate path from viper config.
func casCAPath() string { return viper.GetString("artifact-cas.api-ca") }

// casInsecure returns whether the CAS connection is insecure from viper config.
func casInsecure() bool { return viper.GetBool("api-insecure") }

// CheckAuth verifies the CLI can reach the control plane.
func (e *AttestationExecutor) CheckAuth(_ context.Context) error {
	_, err := NewConfigCurrentContext(&ActionsOpts{
		CPConnection: e.actionOpts.CPConnection,
		Logger:       e.actionOpts.Logger,
		AuthTokenRaw: e.actionOpts.AuthTokenRaw,
	}).Run()
	if err != nil {
		// The wrap keeps the gRPC status reachable with errors.As, so an
		// authentication failure still renders through AuthErrorMessage;
		// anything else is the control plane being unreachable, and its own
		// error says more than a guess would.
		return fmt.Errorf("reaching the chainloop control plane: %w", err)
	}

	return nil
}

// Init starts a new attestation and returns the workflow run ID.
// It uses a local state path to avoid conflicts with concurrent attestations.
// When version is empty the latest project version is used; otherwise the
// attestation targets that specific version.
func (e *AttestationExecutor) Init(ctx context.Context, workflow, project, version string) (string, error) {
	a, err := NewAttestationInit(&AttestationInitOpts{
		ActionsOpts:        e.actionOpts,
		Force:              true,
		LocalStatePath:     e.localStatePath,
		CASURI:             casURI(),
		CASCAPath:          casCAPath(),
		ConnectionInsecure: casInsecure(),
	})
	if err != nil {
		return "", fmt.Errorf("create attestation init action: %w", err)
	}

	runOpts := &AttestationInitRunOpts{
		WorkflowName: workflow,
		ProjectName:  project,
		Collectors:   []string{"aiconfig"},
	}
	if version != "" {
		runOpts.ProjectVersion = version
	} else {
		runOpts.UseLatestVersion = true
	}

	id, err := a.Run(ctx, runOpts)
	if err != nil {
		return "", fmt.Errorf("attestation init: %w", err)
	}

	return id, nil
}

// AddMaterial adds a material of the given kind to the attestation and returns
// the digest it was stored under.
// Empty attestation ID is passed upstream so the crafter uses LocalStatePath
// instead of forcing remote state.
func (e *AttestationExecutor) AddMaterial(ctx context.Context, name, filePath, kind string, annotations map[string]string) (string, error) {
	a, err := NewAttestationAdd(&AttestationAddOpts{
		ActionsOpts:        e.actionOpts,
		LocalStatePath:     e.localStatePath,
		CASURI:             casURI(),
		CASCAPath:          casCAPath(),
		ConnectionInsecure: casInsecure(),
	})
	if err != nil {
		return "", fmt.Errorf("create attestation add action: %w", err)
	}

	added, err := a.Run(ctx, "", name, filePath, kind, annotations, nil, nil)
	if err != nil {
		return "", fmt.Errorf("attestation add %s: %w", kind, err)
	}

	if len(added) == 0 || added[0].Material == nil {
		return "", fmt.Errorf("attestation add %s: no material recorded", kind)
	}

	return added[0].Hash, nil
}

// Reset cancels the current attestation.
func (e *AttestationExecutor) Reset(ctx context.Context, trigger, reason string) error {
	a, err := NewAttestationReset(&AttestationResetOpts{
		ActionsOpts:    e.actionOpts,
		LocalStatePath: e.localStatePath,
	})
	if err != nil {
		return fmt.Errorf("create attestation reset action: %w", err)
	}

	if err := a.Run(ctx, "", trigger, reason); err != nil {
		return fmt.Errorf("attestation reset: %w", err)
	}

	return nil
}

// Push finalizes and pushes the attestation, returning the push result so
// callers can report where the evidence landed.
func (e *AttestationExecutor) Push(ctx context.Context) (*AttestationResult, error) {
	cliVersion, cliDigest, err := e.executableInfo()
	if err != nil {
		return nil, fmt.Errorf("resolve executable info: %w", err)
	}

	a, err := NewAttestationPush(&AttestationPushOpts{
		ActionsOpts:        e.actionOpts,
		LocalStatePath:     e.localStatePath,
		CASURI:             casURI(),
		CASCAPath:          casCAPath(),
		ConnectionInsecure: casInsecure(),
		CLIVersion:         cliVersion,
		CLIDigest:          cliDigest,
	})
	if err != nil {
		return nil, fmt.Errorf("create attestation push action: %w", err)
	}

	res, err := a.Run(ctx, "", nil, false)
	if err != nil {
		return nil, fmt.Errorf("attestation push: %w", err)
	}

	return res, nil
}

// executableInfo returns the CLI version and SHA-256 digest of the running CLI binary
func (e *AttestationExecutor) executableInfo() (string, string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("locating executable: %w", err)
	}

	f, err := os.Open(ex)
	if err != nil {
		return "", "", fmt.Errorf("opening executable: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", "", fmt.Errorf("hashing executable: %w", err)
	}

	return e.cliVersion, fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}
