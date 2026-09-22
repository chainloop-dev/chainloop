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

package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/mocks"
	"github.com/go-kratos/kratos/v2/log"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDownloadServiceAuditEvents(t *testing.T) {
	const (
		backendType = "backend-type"
		// sha256 of "hello world"
		digestHex = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	)

	downloaderClaims := func(sourceInternal bool) *casJWT.Claims {
		return &casJWT.Claims{
			Role:           casJWT.Downloader,
			StoredSecretID: "secret-id",
			BackendType:    backendType,
			OrgID:          testOrgID,
			SourceInternal: sourceInternal,
		}
	}

	tests := []struct {
		name    string
		content string
		claims  *casJWT.Claims
		// removeStagingDir makes staging impossible before the request is served
		removeStagingDir bool
		wantStatus       int
		// wantBodyContains is asserted on the response body when non-empty
		wantBodyContains string
		wantEvents       int
	}{
		{
			name:       "successful download emits an event",
			content:    "hello world",
			claims:     downloaderClaims(false),
			wantStatus: http.StatusOK,
			wantEvents: 1,
		},
		{
			name:             "checksum mismatch is reported as corrupt content, sends no bytes and emits no event",
			content:          "tampered content",
			claims:           downloaderClaims(false),
			wantStatus:       http.StatusInternalServerError,
			wantBodyContains: "does not match the requested digest",
		},
		{
			name:             "staging failure is masked, sends no bytes and emits no event",
			content:          "hello world",
			claims:           downloaderClaims(false),
			removeStagingDir: true,
			wantStatus:       http.StatusInternalServerError,
			wantBodyContains: "server error",
		},
		{
			name:       "internal control plane traffic emits no event",
			content:    "hello world",
			claims:     downloaderClaims(true),
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := mocks.NewProvider(t)
			uploaderDownloader := mocks.NewUploaderDownloader(t)
			provider.On("FromCredentials", mock.Anything, mock.Anything).Return(uploaderDownloader, nil)
			uploaderDownloader.On("Describe", mock.Anything, digestHex).Return(&v1.CASResource{
				FileName: "test.txt", Digest: digestHex, Size: int64(len(tc.content)),
			}, nil)
			if !tc.removeStagingDir {
				uploaderDownloader.On("Download", mock.Anything, mock.Anything, digestHex).Return(nil).
					Run(func(args mock.Arguments) {
						_, err := io.WriteString(args.Get(1).(io.Writer), tc.content)
						require.NoError(t, err)
					})
			}

			stagingDir := t.TempDir()
			if tc.removeStagingDir {
				require.NoError(t, os.RemoveAll(stagingDir))
			}

			audit := &fakePublisher{}
			svc := NewDownloadService(
				backend.Providers{backendType: provider},
				WithLogger(log.DefaultLogger),
				WithAuditDispatcher(newTestDispatcher(audit)),
				WithStagingDir(stagingDir),
			)

			req := httptest.NewRequest(http.MethodGet, "/download/sha256:"+digestHex, nil)
			req = mux.SetURLVars(req, map[string]string{"digest": "sha256:" + digestHex})
			req = req.WithContext(jwtMiddleware.NewContext(req.Context(), tc.claims))

			w := httptest.NewRecorder()
			svc.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			if tc.wantStatus == http.StatusOK {
				assert.Equal(t, tc.content, w.Body.String())
				assert.Equal(t, strconv.Itoa(len(tc.content)), w.Header().Get("Content-Length"))
				assert.Equal(t, "attachment; filename=test.txt", w.Header().Get("Content-Disposition"))
			} else {
				assert.NotContains(t, w.Body.String(), tc.content, "no unverified byte may reach the client")
				assert.Contains(t, w.Body.String(), tc.wantBodyContains)
			}
			if !tc.removeStagingDir {
				// the staging dir is left clean on every exit path
				entries, err := os.ReadDir(stagingDir)
				require.NoError(t, err)
				assert.Empty(t, entries)
			}
			require.Len(t, audit.published, tc.wantEvents)
			if tc.wantEvents == 0 {
				return
			}

			info := decodeArtifactEvent(t, audit.published[0])
			assert.Equal(t, digestHex, info.Digest)
			assert.Equal(t, int64(len(tc.content)), info.SizeBytes)
			assert.Equal(t, "test.txt", info.FileName)
			assert.Equal(t, backendType, info.BackendType)
			assert.False(t, info.Skipped)
		})
	}
}
