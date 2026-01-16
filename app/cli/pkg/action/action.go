//
// Copyright 2024-2025 The Chainloop Authors.
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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	clientAPI "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/statemanager/filesystem"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/statemanager/remote"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/chainloop-dev/chainloop/pkg/grpcconn"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

const (
	PolicyViolationBlockingStrategyEnforced = "ENFORCED"
	PolicyViolationBlockingStrategyAdvisory = "ADVISORY"
)

type ActionsOpts struct {
	CPConnection *grpc.ClientConn
	Logger       zerolog.Logger
	AuthTokenRaw string
	OutputFormat string
}

type OffsetPagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalPages int `json:"totalPages"`
	TotalCount int `json:"totalCount"`
}

func toTimePtr(t time.Time) *time.Time {
	return &t
}

// load a crafter with either local or remote state

type newCrafterStateOpts struct {
	enableRemoteState bool
	localStatePath    string
}

func newCrafter(stateOpts *newCrafterStateOpts, conn *grpc.ClientConn, opts ...crafter.NewOpt) (*crafter.Crafter, error) {
	var stateManager crafter.StateManager
	var err error

	if stateOpts == nil {
		return nil, fmt.Errorf("missing state manager options")
	}

	// run opts to extract logger
	c := &crafter.Crafter{}
	for _, opt := range opts {
		_ = opt(c)
	}

	switch stateOpts.enableRemoteState {
	case true:
		stateManager, err = remote.New(pb.NewAttestationStateServiceClient(conn), c.Logger)
	case false:
		attestationStatePath := filepath.Join(os.TempDir(), "chainloop-attestation.tmp.json")
		if path := stateOpts.localStatePath; path != "" {
			attestationStatePath = path
		}

		c.Logger.Debug().Str("path", fmt.Sprintf("file:%s", attestationStatePath)).Msg("using local state")
		stateManager, err = filesystem.New(attestationStatePath)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	attClient := pb.NewAttestationServiceClient(conn)

	return crafter.NewCrafter(stateManager, attClient, opts...)
}

// getCASBackend tries to get CAS upload credentials and set up a CAS client
func getCASBackend(ctx context.Context, client pb.AttestationServiceClient, workflowRunID, casCAPath, casURI string, casConnectionInsecure bool, logger zerolog.Logger, casBackend *casclient.CASBackend) (*clientAPI.Attestation_CASBackend, func() error, error) {
	credsResp, err := client.GetUploadCreds(ctx, &pb.AttestationServiceGetUploadCredsRequest{
		WorkflowRunId: workflowRunID,
	})
	if err != nil {
		// Log warning but don't fail - will fall back to inline storage
		logger.Warn().Err(err).Msg("failed to get CAS credentials for PR metadata, will store inline")
		return nil, nil, fmt.Errorf("getting upload creds: %w", err)
	}

	if credsResp == nil || credsResp.GetResult() == nil {
		logger.Debug().Msg("no upload creds result, will store inline")
		return nil, nil, fmt.Errorf("getting upload creds: %w", err)
	}

	result := credsResp.GetResult()
	backend := result.GetBackend()
	if backend == nil {
		logger.Debug().Msg("no backend info in upload creds, will store inline")
		return nil, nil, fmt.Errorf("no backend found in upload creds")
	}

	casBackendInfo := &clientAPI.Attestation_CASBackend{
		CasBackendId:   backend.Id,
		CasBackendName: backend.Name,
		Fallback:       backend.Fallback,
	}

	casBackend.Name = backend.Provider
	if backend.GetLimits() != nil {
		casBackend.MaxSize = backend.GetLimits().MaxBytes
	}

	// Only attempt to create a CAS connection when not inline and token is present
	if backend.IsInline || result.Token == "" {
		return casBackendInfo, nil, nil
	}

	opts := []grpcconn.Option{grpcconn.WithInsecure(casConnectionInsecure)}
	if casCAPath != "" {
		opts = append(opts, grpcconn.WithCAFile(casCAPath))
	}

	artifactCASConn, err := grpcconn.New(casURI, result.Token, opts...)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to create CAS connection, will store inline")
		return nil, nil, fmt.Errorf("creating CAS connection: %w", err)
	}

	casBackend.Uploader = casclient.New(artifactCASConn, casclient.WithLogger(logger))
	return casBackendInfo, artifactCASConn.Close, nil
}

// fetchUIDashboardURL retrieves the UI Dashboard URL from the control plane
// Returns empty string if not configured or if fetch fails
func fetchUIDashboardURL(ctx context.Context, cpConnection *grpc.ClientConn) string {
	if cpConnection == nil {
		return ""
	}

	tmoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client := pb.NewStatusServiceClient(cpConnection)
	resp, err := client.Infoz(tmoutCtx, &pb.InfozRequest{})
	if err != nil {
		return ""
	}

	return resp.UiDashboardUrl
}

// buildAttestationViewURL constructs the attestation view URL
// Returns empty string if platformURL is not configured
func buildAttestationViewURL(uiDashboardURL, digest string) string {
	if uiDashboardURL == "" || digest == "" {
		return ""
	}

	// Trim trailing slash from platform URL if present
	uiDashboardURL = strings.TrimRight(uiDashboardURL, "/")
	return fmt.Sprintf("%s/attestation/%s?tab=summary", uiDashboardURL, digest)
}
