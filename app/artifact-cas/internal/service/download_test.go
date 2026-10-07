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

const (
	downloadBackendType = "backend-type"
	downloadContent     = "hello world"
	// sha256 of downloadContent
	downloadDigestHex = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	downloadFileName  = "test.txt"
)

func TestDownloadServiceAuditEvents(t *testing.T) {
	const (
		backendType = downloadBackendType
		digestHex   = downloadDigestHex
	)

	downloaderClaims := func(sourceInternal bool) *casJWT.Claims {
		return &casJWT.Claims{
			Role:           casJWT.Downloader,
			StoredSecretID: testStoredSecretID,
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
			content:    downloadContent,
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
			content:          downloadContent,
			claims:           downloaderClaims(false),
			removeStagingDir: true,
			wantStatus:       http.StatusInternalServerError,
			wantBodyContains: "server error",
		},
		{
			name:       "internal control plane traffic emits no event",
			content:    downloadContent,
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
				FileName: downloadFileName, Digest: digestHex, Size: int64(len(tc.content)),
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
				assert.Equal(t, `"sha256:`+digestHex+`"`, w.Header().Get("ETag"))
				assert.Equal(t, "private, no-cache", w.Header().Get("Cache-Control"))
			} else {
				assert.Empty(t, w.Header().Get("ETag"), "a failed download must not be cached")
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
			assert.Equal(t, downloadFileName, info.FileName)
			assert.Equal(t, backendType, info.BackendType)
			assert.False(t, info.Skipped)
		})
	}
}

func TestDownloadServiceConditionalRequests(t *testing.T) {
	const (
		backendType = downloadBackendType
		content     = downloadContent
		digestHex   = downloadDigestHex
		etag        = `"sha256:` + digestHex + `"`
	)

	claims := &casJWT.Claims{
		Role:           casJWT.Downloader,
		StoredSecretID: testStoredSecretID,
		BackendType:    backendType,
		OrgID:          testOrgID,
	}

	tests := []struct {
		name        string
		ifNoneMatch string
		claims      *casJWT.Claims
		// describeErr is returned by the backend metadata call
		describeErr error
		// wantDownload means the backend object is copied
		wantDownload bool
		wantStatus   int
		wantEvents   int
	}{
		{
			name:        "matching etag answers not modified without copying the object",
			ifNoneMatch: etag,
			claims:      claims,
			wantStatus:  http.StatusNotModified,
		},
		{
			name:        "weak etag in a list matches",
			ifNoneMatch: `"sha256:other", W/` + etag,
			claims:      claims,
			wantStatus:  http.StatusNotModified,
		},
		{
			name:        "wildcard matches",
			ifNoneMatch: "*",
			claims:      claims,
			wantStatus:  http.StatusNotModified,
		},
		{
			name:         "unquoted digest does not match",
			ifNoneMatch:  "sha256:" + digestHex,
			claims:       claims,
			wantDownload: true,
			wantStatus:   http.StatusOK,
			wantEvents:   1,
		},
		{
			name:         "different etag downloads the object",
			ifNoneMatch:  `"sha256:other"`,
			claims:       claims,
			wantDownload: true,
			wantStatus:   http.StatusOK,
			wantEvents:   1,
		},
		{
			name:        "matching etag without a token is unauthorized",
			ifNoneMatch: etag,
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:        "matching etag for an object missing in the backend is not found",
			ifNoneMatch: etag,
			claims:      claims,
			describeErr: backend.NewErrNotFound("artifact"),
			wantStatus:  http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := mocks.NewProvider(t)
			if tc.claims != nil {
				uploaderDownloader := mocks.NewUploaderDownloader(t)
				provider.On("FromCredentials", mock.Anything, mock.Anything).Return(uploaderDownloader, nil)
				var resource *v1.CASResource
				if tc.describeErr == nil {
					resource = &v1.CASResource{FileName: downloadFileName, Digest: digestHex, Size: int64(len(content))}
				}
				uploaderDownloader.On("Describe", mock.Anything, digestHex).Return(resource, tc.describeErr)
				// the mock fails the test on any Download call it does not expect
				if tc.wantDownload {
					uploaderDownloader.On("Download", mock.Anything, mock.Anything, digestHex).Return(nil).
						Run(func(args mock.Arguments) {
							_, err := io.WriteString(args.Get(1).(io.Writer), content)
							require.NoError(t, err)
						})
				}
			}

			audit := &fakePublisher{}
			svc := NewDownloadService(
				backend.Providers{backendType: provider},
				WithLogger(log.DefaultLogger),
				WithAuditDispatcher(newTestDispatcher(audit)),
				WithStagingDir(t.TempDir()),
			)

			req := httptest.NewRequest(http.MethodGet, "/download/sha256:"+digestHex, nil)
			req = mux.SetURLVars(req, map[string]string{"digest": "sha256:" + digestHex})
			req.Header.Set("If-None-Match", tc.ifNoneMatch)
			if tc.claims != nil {
				req = req.WithContext(jwtMiddleware.NewContext(req.Context(), tc.claims))
			}

			w := httptest.NewRecorder()
			svc.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			switch tc.wantStatus {
			case http.StatusNotModified:
				assert.Empty(t, w.Body.String())
				assert.Equal(t, etag, w.Header().Get("ETag"))
				assert.Equal(t, "private, no-cache", w.Header().Get("Cache-Control"))
			case http.StatusOK:
				assert.Equal(t, content, w.Body.String())
				assert.Equal(t, etag, w.Header().Get("ETag"))
			default:
				assert.Empty(t, w.Header().Get("ETag"))
			}
			assert.Len(t, audit.published, tc.wantEvents)
		})
	}
}
