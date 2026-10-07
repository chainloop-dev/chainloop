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
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	// gitlabStatusVerified is the status of a signature made with a key of the commit author
	gitlabStatusVerified = "verified"
	// gitlabStatusVerifiedSystem is the status of a signature made by GitLab itself, for
	// example for commits created in the web UI or when merging a merge request
	gitlabStatusVerifiedSystem = "verified_system"
	// gitlabSignatureNotFound is the error message GitLab returns for an unsigned commit
	gitlabSignatureNotFound = "Signature Not Found"
	// maxGitLabErrorBodySize limits how much of an error response body is read
	maxGitLabErrorBodySize = 64 * 1024
)

// GitLabCredentials holds the tokens that can authenticate calls to the GitLab API
type GitLabCredentials struct {
	// APIToken is a personal, project or group access token with the read_api scope.
	// It is required to verify commits of private and internal projects.
	APIToken string
	// JobToken is the CI/CD job token. GitLab does not accept job tokens on the commit
	// signature endpoint and handles the request as anonymous, which only works for public projects.
	JobToken string
}

// GitLabCredentialsFromEnv reads the GitLab API credentials from GITLAB_TOKEN and CI_JOB_TOKEN
func GitLabCredentialsFromEnv() GitLabCredentials {
	return GitLabCredentials{
		APIToken: os.Getenv("GITLAB_TOKEN"),
		JobToken: os.Getenv("CI_JOB_TOKEN"),
	}
}

// VerifyGitLabCommit verifies a commit signature using the GitLab API
func VerifyGitLabCommit(ctx context.Context, baseURL, projectPath, commitHash string, credentials GitLabCredentials, logger *zerolog.Logger) *CommitVerification {
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
		return &CommitVerification{
			Attempted: true,
			Status:    VerificationStatusUnavailable,
			Reason:    fmt.Sprintf("Failed to create request: %v", err),
			Platform:  "gitlab",
		}
	}

	// Set headers
	switch {
	case credentials.APIToken != "":
		req.Header.Set("PRIVATE-TOKEN", credentials.APIToken)
	case credentials.JobToken != "":
		req.Header.Set("JOB-TOKEN", credentials.JobToken)
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Debug().Err(err).Str("commit", commitHash).Msg("failed to fetch commit from GitLab")
		}
		return &CommitVerification{
			Attempted: true,
			Status:    VerificationStatusUnavailable,
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

		// GitLab also answers 404 when the project or the commit is not visible to the
		// caller, so only the "Signature Not Found" message means the commit is unsigned
		if resp.StatusCode == http.StatusNotFound {
			message := gitlabErrorMessage(resp.Body)
			if strings.Contains(message, gitlabSignatureNotFound) {
				return &CommitVerification{
					Attempted: true,
					Status:    VerificationStatusNotApplicable,
					Reason:    "Commit is not signed",
					Platform:  "gitlab",
				}
			}

			if message == "" {
				message = "HTTP 404"
			}
			reason := fmt.Sprintf("GitLab API error: %s", message)
			if credentials.APIToken == "" {
				reason += " (set GITLAB_TOKEN to a token with the read_api scope to verify commits of private and internal projects)"
			}
			return &CommitVerification{
				Attempted: true,
				Status:    VerificationStatusUnavailable,
				Reason:    reason,
				Platform:  "gitlab",
			}
		}

		var reason string
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			reason = "GitLab API authentication failed"
		} else {
			reason = fmt.Sprintf("GitLab API error: HTTP %d", resp.StatusCode)
		}
		return &CommitVerification{
			Attempted: true,
			Status:    VerificationStatusUnavailable,
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
		return &CommitVerification{
			Attempted: true,
			Status:    VerificationStatusUnavailable,
			Reason:    fmt.Sprintf("Failed to parse response: %v", err),
			Platform:  "gitlab",
		}
	}

	// Parse GitLab verification status
	var status VerificationStatus
	var reason string
	var keyID string
	var signatureAlgorithm string

	if signatureResponse.VerificationStatus == gitlabStatusVerified || signatureResponse.VerificationStatus == gitlabStatusVerifiedSystem {
		status = VerificationStatusVerified
		reason = "Commit signed and verified"
		if signatureResponse.VerificationStatus == gitlabStatusVerifiedSystem {
			reason = "Commit signed by GitLab and verified"
		}
		if signatureResponse.GPGKeyID != 0 {
			keyID = fmt.Sprintf("%d", signatureResponse.GPGKeyID)
		} else if signatureResponse.GPGKeyPrimaryKeyID != "" {
			keyID = signatureResponse.GPGKeyPrimaryKeyID
		}
		signatureAlgorithm = signatureResponse.SignatureType
	} else {
		status = VerificationStatusUnverified
		reason = fmt.Sprintf("Signature not verified: %s", signatureResponse.VerificationStatus)
		if signatureResponse.GPGKeyID != 0 {
			keyID = fmt.Sprintf("%d", signatureResponse.GPGKeyID)
		}
		signatureAlgorithm = signatureResponse.SignatureType
	}

	if logger != nil {
		logger.Debug().Int("status", int(status)).Str("reason", reason).Str("verification_status", signatureResponse.VerificationStatus).Msg("GitLab commit verification completed")
	}

	return &CommitVerification{
		Attempted:          true,
		Status:             status,
		Reason:             reason,
		Platform:           "gitlab",
		KeyID:              keyID,
		SignatureAlgorithm: signatureAlgorithm,
	}
}

// gitlabErrorMessage returns the message of a GitLab API error response, or an empty
// string when the body is not a GitLab error
func gitlabErrorMessage(body io.Reader) string {
	var errorResponse struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(body, maxGitLabErrorBodySize)).Decode(&errorResponse); err != nil {
		return ""
	}

	return errorResponse.Message
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
