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
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// VerifyGitHubCommit verifies a commit signature using the GitHub API
func VerifyGitHubCommit(ctx context.Context, owner, repo, commitHash, token string, logger *zerolog.Logger) *CommitVerification {
	// Build API URL
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repo, commitHash)

	// Create HTTP client with timeout
	client := &http.Client{Timeout: 10 * time.Second}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		if logger != nil {
			logger.Debug().Err(err).Msg("failed to create GitHub API request")
		}
		return &CommitVerification{
			Attempted: true,
			Status:    VerificationStatusUnavailable,
			Reason:    fmt.Sprintf("Failed to create request: %v", err),
			Platform:  "github",
		}
	}

	// Set headers
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Debug().Err(err).Str("commit", commitHash).Msg("failed to fetch commit from GitHub")
		}
		return &CommitVerification{
			Attempted: true,
			Status:    VerificationStatusUnavailable,
			Reason:    fmt.Sprintf("GitHub API error: %v", err),
			Platform:  "github",
		}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		if logger != nil {
			logger.Debug().Int("status", resp.StatusCode).Str("commit", commitHash).Msg("GitHub API returned non-OK status")
		}
		var reason string
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			reason = "GitHub API authentication failed"
		case http.StatusNotFound:
			reason = "Commit not found"
		default:
			reason = fmt.Sprintf("GitHub API error: HTTP %d", resp.StatusCode)
		}
		return &CommitVerification{
			Attempted: true,
			Status:    VerificationStatusUnavailable,
			Reason:    reason,
			Platform:  "github",
		}
	}

	// Parse response
	var commitResponse githubCommitResponse
	if err := json.NewDecoder(resp.Body).Decode(&commitResponse); err != nil {
		if logger != nil {
			logger.Debug().Err(err).Msg("failed to decode GitHub API response")
		}
		return &CommitVerification{
			Attempted: true,
			Status:    VerificationStatusUnavailable,
			Reason:    fmt.Sprintf("Failed to parse response: %v", err),
			Platform:  "github",
		}
	}

	// Check if verification info is available
	if commitResponse.Commit.Verification == nil {
		return &CommitVerification{
			Attempted: true,
			Status:    VerificationStatusNotApplicable,
			Reason:    "No signature verification data available",
			Platform:  "github",
		}
	}

	verification := commitResponse.Commit.Verification

	// Parse GitHub verification status
	var status VerificationStatus
	if verification.Verified {
		status = VerificationStatusVerified
	} else {
		status = VerificationStatusUnverified
	}

	// Detect signature type from the signature content
	signatureAlgorithm := detectSignatureType(verification.Signature)

	if logger != nil {
		logger.Debug().Int("status", int(status)).Str("reason", verification.Reason).Bool("verified", verification.Verified).Str("signature_type", signatureAlgorithm).Msg("GitHub commit verification completed")
	}

	return &CommitVerification{
		Attempted:          true,
		Status:             status,
		Reason:             verification.Reason,
		Platform:           "github",
		SignatureAlgorithm: signatureAlgorithm,
	}
}

// detectSignatureType inspects the signature content to determine its type
// GitHub supports GPG, SSH, and S/MIME signatures
// Format references:
// - Git documentation: https://git-scm.com/docs/gitformat-signature
// - SSH format: https://blog.gitbutler.com/signing-commits-in-git-explained
func detectSignatureType(signature string) string {
	if signature == "" {
		return ""
	}

	// Trim whitespace for consistent detection
	sig := strings.TrimSpace(signature)

	// GPG/PGP signatures
	// Format: -----BEGIN PGP SIGNATURE-----
	if strings.HasPrefix(sig, "-----BEGIN PGP SIGNATURE-----") {
		return "PGP"
	}

	// SSH signatures
	// Format: -----BEGIN SSH SIGNATURE-----
	if strings.HasPrefix(sig, "-----BEGIN SSH SIGNATURE-----") {
		return "SSH"
	}

	// X.509/S/MIME signatures
	// Format: -----BEGIN SIGNED MESSAGE-----
	if strings.HasPrefix(sig, "-----BEGIN SIGNED MESSAGE-----") {
		return "X509"
	}

	// RFC1991 PGP format (legacy)
	if strings.HasPrefix(sig, "-----BEGIN PGP MESSAGE-----") {
		return "PGP"
	}

	// Unknown signature format
	return "UNKNOWN"
}

// githubCommitResponse represents the GitHub API response for commit details
type githubCommitResponse struct {
	Commit struct {
		Verification *githubVerification `json:"verification"`
	} `json:"commit"`
}

// githubVerification represents GitHub's verification information
type githubVerification struct {
	Verified  bool   `json:"verified"`
	Reason    string `json:"reason"`
	Signature string `json:"signature"`
	Payload   string `json:"payload"`
}
