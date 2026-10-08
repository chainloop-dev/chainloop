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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCommitHash = "ea3a974660a030f0314aa1505a64792bf2633b33"
	testAPIToken   = "api-token"
	testJobToken   = "job-token"
)

// Response bodies as returned by the GitLab API
const (
	gitlabSSHVerifiedSystemBody = `{"signature_type":"SSH","verification_status":"verified_system","key":{"id":14178941,"title":"Gitlab Signing","usage_type":"signing"},"commit_source":"gitaly"}`
	gitlabPGPVerifiedBody       = `{"signature_type":"PGP","verification_status":"verified","gpg_key_id":1,"gpg_key_primary_keyid":"8254AAB3FBD54AC9","gpg_key_user_name":"John Doe","gpg_key_user_email":"johndoe@example.com","gpg_key_subkey_id":null,"commit_source":"gitaly"}`
	gitlabPGPUnverifiedKeyBody  = `{"signature_type":"PGP","verification_status":"unverified_key","gpg_key_id":2,"commit_source":"gitaly"}`
)

func TestVerifyGitLabCommit(t *testing.T) {
	testCases := []struct {
		name          string
		statusCode    int
		body          string
		credentials   GitLabCredentials
		wantStatus    VerificationStatus
		wantAlgorithm string
		wantKeyID     string
		wantReason    []string
	}{
		{
			name:          "pgp signature verified",
			statusCode:    http.StatusOK,
			body:          gitlabPGPVerifiedBody,
			wantStatus:    VerificationStatusVerified,
			wantAlgorithm: "PGP",
			wantKeyID:     "1",
		},
		{
			name:          "ssh signature made by GitLab is verified",
			statusCode:    http.StatusOK,
			body:          gitlabSSHVerifiedSystemBody,
			wantStatus:    VerificationStatusVerified,
			wantAlgorithm: "SSH",
		},
		{
			name:          "pgp signature with an unverified key",
			statusCode:    http.StatusOK,
			body:          gitlabPGPUnverifiedKeyBody,
			wantStatus:    VerificationStatusUnverified,
			wantAlgorithm: "PGP",
			wantKeyID:     "2",
			wantReason:    []string{"unverified_key"},
		},
		{
			name:       "commit is not signed",
			statusCode: http.StatusNotFound,
			body:       `{"message":"404 Signature Not Found"}`,
			wantStatus: VerificationStatusNotApplicable,
		},
		{
			// GitLab answers 404 to anonymous requests for private and internal projects
			name:       "project not visible without an API token",
			statusCode: http.StatusNotFound,
			body:       `{"message":"404 Project Not Found"}`,
			wantStatus: VerificationStatusUnavailable,
			wantReason: []string{"404 Project Not Found", "GITLAB_TOKEN"},
		},
		{
			name:        "project not visible with an API token",
			statusCode:  http.StatusNotFound,
			body:        `{"message":"404 Project Not Found"}`,
			credentials: GitLabCredentials{APIToken: testAPIToken},
			wantStatus:  VerificationStatusUnavailable,
			wantReason:  []string{"404 Project Not Found"},
		},
		{
			name:       "commit not known to GitLab",
			statusCode: http.StatusNotFound,
			body:       `{"message":"404 Commit Not Found"}`,
			wantStatus: VerificationStatusUnavailable,
			wantReason: []string{"404 Commit Not Found"},
		},
		{
			name:        "invalid API token",
			statusCode:  http.StatusUnauthorized,
			body:        `{"message":"401 Unauthorized"}`,
			credentials: GitLabCredentials{APIToken: testAPIToken},
			wantStatus:  VerificationStatusUnavailable,
			wantReason:  []string{"authentication failed"},
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"message":"500 Internal Server Error"}`,
			wantStatus: VerificationStatusUnavailable,
			wantReason: []string{"HTTP 500"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, fmt.Sprintf("/api/v4/projects/group%%2Fproject/repository/commits/%s/signature", testCommitHash), r.URL.RawPath)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()

			got := VerifyGitLabCommit(context.Background(), server.URL, "group/project", testCommitHash, tc.credentials, nil)
			require.NotNil(t, got)

			assert.True(t, got.Attempted)
			assert.Equal(t, "gitlab", got.Platform)
			assert.Equal(t, tc.wantStatus, got.Status)
			assert.Equal(t, tc.wantAlgorithm, got.SignatureAlgorithm)
			assert.Equal(t, tc.wantKeyID, got.KeyID)
			for _, want := range tc.wantReason {
				assert.Contains(t, got.Reason, want)
			}
		})
	}
}

func TestVerifyGitLabCommitAuthHeaders(t *testing.T) {
	testCases := []struct {
		name             string
		credentials      GitLabCredentials
		wantPrivateToken string
		wantJobToken     string
	}{
		{
			name: "no credentials",
		},
		{
			name:             "api token",
			credentials:      GitLabCredentials{APIToken: testAPIToken},
			wantPrivateToken: testAPIToken,
		},
		{
			name:         "job token",
			credentials:  GitLabCredentials{JobToken: testJobToken},
			wantJobToken: testJobToken,
		},
		{
			// GitLab does not accept job tokens on the signature endpoint, so the API token wins
			name:             "api token and job token",
			credentials:      GitLabCredentials{APIToken: testAPIToken, JobToken: testJobToken},
			wantPrivateToken: testAPIToken,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.wantPrivateToken, r.Header.Get("PRIVATE-TOKEN"))
				assert.Equal(t, tc.wantJobToken, r.Header.Get("JOB-TOKEN"))
				fmt.Fprint(w, gitlabPGPVerifiedBody)
			}))
			defer server.Close()

			got := VerifyGitLabCommit(context.Background(), server.URL, "group/project", testCommitHash, tc.credentials, nil)
			assert.Equal(t, VerificationStatusVerified, got.Status)
		})
	}
}

func TestVerifyGitLabCommitUntrustedCertificate(t *testing.T) {
	// The server certificate is not signed by a CA in the system pool, like a
	// self-managed GitLab instance that uses a private CA
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, gitlabPGPVerifiedBody)
	}))
	defer server.Close()

	got := VerifyGitLabCommit(context.Background(), server.URL, "group/project", testCommitHash, GitLabCredentials{}, nil)

	assert.Equal(t, VerificationStatusUnavailable, got.Status)
	assert.Contains(t, got.Reason, "certificate")
}

func TestGitLabCredentialsFromEnv(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", testAPIToken)
	t.Setenv("CI_JOB_TOKEN", testJobToken)

	assert.Equal(t, GitLabCredentials{APIToken: testAPIToken, JobToken: testJobToken}, GitLabCredentialsFromEnv())
}
