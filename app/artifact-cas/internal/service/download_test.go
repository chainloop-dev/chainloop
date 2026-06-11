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
		name       string
		content    string
		claims     *casJWT.Claims
		wantStatus int
		wantEvents int
	}{
		{
			name:       "successful download emits an event",
			content:    "hello world",
			claims:     downloaderClaims(false),
			wantStatus: http.StatusOK,
			wantEvents: 1,
		},
		{
			name:       "checksum mismatch emits no event",
			content:    "tampered content",
			claims:     downloaderClaims(false),
			wantStatus: http.StatusUnauthorized,
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
			uploaderDownloader.On("Download", mock.Anything, mock.Anything, digestHex).Return(nil).
				Run(func(args mock.Arguments) {
					_, err := io.WriteString(args.Get(1).(io.Writer), tc.content)
					require.NoError(t, err)
				})

			audit := &fakePublisher{}
			svc := NewDownloadService(
				backend.Providers{backendType: provider},
				WithLogger(log.DefaultLogger),
				WithAuditDispatcher(newTestDispatcher(audit)),
			)

			req := httptest.NewRequest(http.MethodGet, "/download/sha256:"+digestHex, nil)
			req = mux.SetURLVars(req, map[string]string{"digest": "sha256:" + digestHex})
			req = req.WithContext(jwtMiddleware.NewContext(req.Context(), tc.claims))

			w := httptest.NewRecorder()
			svc.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
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
