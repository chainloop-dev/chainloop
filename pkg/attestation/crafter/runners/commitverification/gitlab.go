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

package commitverification

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/rs/zerolog"
)

// VerifyGitLabCommit verifies a commit signature using the GitLab API
func VerifyGitLabCommit(ctx context.Context, baseURL, projectPath, commitHash, token string, logger *zerolog.Logger) *api.Commit_CommitVerification {
	// URL encode the project path (e.g., "group/project" -> "group%2Fproject")
	encodedProject := url.PathEscape(projectPath)

	// Build API URL - use the dedicated signature endpoint
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/commits/%s/signature", baseURL, encodedProject, commitHash)

	// Create HTTP client with timeout
	client := &http.Client{Timeout: 10 * time.Second}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		if logger != nil {
			logger.Debug().Err(err).Msg("failed to create GitLab API request")
		}
		return &api.Commit_CommitVerification{
			Attempted: true,
			Status:    api.Commit_CommitVerification_VERIFICATION_STATUS_UNAVAILABLE,
			Reason:    fmt.Sprintf("Failed to create request: %v", err),
			Platform:  "gitlab",
		}
	}

	// Set headers
	if token != "" {
		req.Header.Set("JOB-TOKEN", token)
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Debug().Err(err).Str("commit", commitHash).Msg("failed to fetch commit from GitLab")
		}
		return &api.Commit_CommitVerification{
			Attempted: true,
			Status:    api.Commit_CommitVerification_VERIFICATION_STATUS_UNAVAILABLE,
			Reason:    fmt.Sprintf("GitLab API error: %v", err),
			Platform:  "gitlab",
		}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		if logger != nil {
			logger.Debug().Int("status", resp.StatusCode).Str("commit", commitHash).Msg("GitLab API returned non-OK status")
		}

		// 404 means the commit is unsigned (no signature data available)
		if resp.StatusCode == http.StatusNotFound {
			return &api.Commit_CommitVerification{
				Attempted: true,
				Status:    api.Commit_CommitVerification_VERIFICATION_STATUS_NOT_APPLICABLE,
				Reason:    "Commit is not signed",
				Platform:  "gitlab",
			}
		}

		var reason string
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			reason = "GitLab API authentication failed"
		} else {
			reason = fmt.Sprintf("GitLab API error: HTTP %d", resp.StatusCode)
		}
		return &api.Commit_CommitVerification{
			Attempted: true,
			Status:    api.Commit_CommitVerification_VERIFICATION_STATUS_UNAVAILABLE,
			Reason:    reason,
			Platform:  "gitlab",
		}
	}

	// Parse response
	var signatureResponse gitlabCommitResponse
	if err := json.NewDecoder(resp.Body).Decode(&signatureResponse); err != nil {
		if logger != nil {
			logger.Debug().Err(err).Msg("failed to decode GitLab API response")
		}
		return &api.Commit_CommitVerification{
			Attempted: true,
			Status:    api.Commit_CommitVerification_VERIFICATION_STATUS_UNAVAILABLE,
			Reason:    fmt.Sprintf("Failed to parse response: %v", err),
			Platform:  "gitlab",
		}
	}

	// Parse GitLab verification status
	var status api.Commit_CommitVerification_VerificationStatus
	var reason string
	var keyID string
	var signatureAlgorithm string

	if signatureResponse.VerificationStatus == "verified" {
		status = api.Commit_CommitVerification_VERIFICATION_STATUS_VERIFIED
		reason = "Commit signed and verified"
		if signatureResponse.GPGKeyID != 0 {
			keyID = fmt.Sprintf("%d", signatureResponse.GPGKeyID)
		} else if signatureResponse.GPGKeyPrimaryKeyID != "" {
			keyID = signatureResponse.GPGKeyPrimaryKeyID
		}
		signatureAlgorithm = signatureResponse.SignatureType
	} else {
		status = api.Commit_CommitVerification_VERIFICATION_STATUS_UNVERIFIED
		reason = fmt.Sprintf("Signature not verified: %s", signatureResponse.VerificationStatus)
		if signatureResponse.GPGKeyID != 0 {
			keyID = fmt.Sprintf("%d", signatureResponse.GPGKeyID)
		}
		signatureAlgorithm = signatureResponse.SignatureType
	}

	if logger != nil {
		logger.Debug().Str("status", status.String()).Str("reason", reason).Str("verification_status", signatureResponse.VerificationStatus).Msg("GitLab commit verification completed")
	}

	return &api.Commit_CommitVerification{
		Attempted:          true,
		Status:             status,
		Reason:             reason,
		Platform:           "gitlab",
		KeyId:              keyID,
		SignatureAlgorithm: signatureAlgorithm,
	}
}

// gitlabCommitResponse represents the GitLab API response for commit signature
// from the /signature endpoint (not the general commits endpoint)
type gitlabCommitResponse struct {
	SignatureType      string `json:"signature_type"`
	VerificationStatus string `json:"verification_status"`
	GPGKeyID           int    `json:"gpg_key_id"`
	GPGKeyPrimaryKeyID string `json:"gpg_key_primary_keyid"`
	GPGKeyUserName     string `json:"gpg_key_user_name"`
	GPGKeyUserEmail    string `json:"gpg_key_user_email"`
	GPGKeySubkeyID     string `json:"gpg_key_subkey_id"`
	CommitSource       string `json:"commit_source"`
}
