//
// Copyright 2023 The Chainloop Authors.
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

package oci

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *testSuite) TestUpload() {
	s.T().Run("invalid repo", func(t *testing.T) {
		b, err := NewBackend("bogus-registry", &RegistryOptions{})
		require.NoError(s.T(), err)
		err = b.Upload(context.TODO(), bytes.NewBuffer(s.uploadedContent), s.casResource)
		assert.Error(s.T(), err)
	})

	s.T().Run("empty content", func(t *testing.T) {
		err := s.validBackend.Upload(context.TODO(), bytes.NewBuffer(nil), s.casResource)
		assert.Error(s.T(), err)
		assert.ErrorContains(s.T(), err, "content is empty")
	})

	s.T().Run("digest mistmatch", func(t *testing.T) {
		r := &pb.CASResource{Digest: "sha256:deadbeef", FileName: "test.txt"}
		err := s.validBackend.Upload(context.TODO(), bytes.NewBuffer(s.uploadedContent), r)
		assert.ErrorContains(s.T(), err, "layer digest does not match")
	})

	s.T().Run("success", func(t *testing.T) {
		err := s.validBackend.Upload(context.TODO(), bytes.NewBuffer(s.uploadedContent), s.casResource)
		assert.NoError(s.T(), err)
	})
}

func (s *testSuite) TestExists() {
	assert := assert.New(s.T())
	// Valid image
	err := s.validBackend.Upload(context.TODO(), bytes.NewBuffer(s.uploadedContent), s.casResource)
	require.NoError(s.T(), err)
	// Image not uploaded by us
	digestOthers := "another-deadbeef"
	ref, err := name.ParseReference(s.validBackend.resourcePath(digestOthers))
	require.NoError(s.T(), err)
	err = remote.Write(ref, empty.Image, remote.WithAuthFromKeychain(s.validBackend.keychain))
	require.NoError(s.T(), err)

	testCases := []struct {
		name    string
		digest  string
		wantErr bool
		errMsg  string
		want    bool
	}{
		{
			name:    "empty digest",
			digest:  "",
			wantErr: true,
			errMsg:  "digest is empty",
		},
		{
			name:   "image not found",
			digest: "deadbeef",
			want:   false,
		},
		{
			name:   "image already exists",
			digest: s.casResource.Digest,
			want:   true,
		},
		{
			name:   "image exists but not uploaded by us",
			digest: digestOthers,
			want:   false,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			g, err := s.validBackend.Exists(context.Background(), tc.digest)
			if tc.wantErr {
				assert.ErrorContains(err, tc.errMsg)
			} else {
				assert.NoError(err)
				assert.Equal(tc.want, g)
			}
		})
	}
}

func (s *testSuite) TestDescribe() {
	testCases := []struct {
		name    string
		digest  string
		wantErr bool
		errMsg  string
		want    *pb.CASResource
	}{
		{
			name:    "empty digest",
			digest:  "",
			wantErr: true,
			errMsg:  "digest is empty",
		},
		{
			name:    "not found",
			digest:  "deadbeef",
			wantErr: true,
			errMsg:  "Unknown name",
		},
		{
			name:   "valid image",
			digest: s.casResource.Digest,
			want:   s.casResource,
		},
	}

	assert := assert.New(s.T())
	err := s.validBackend.Upload(context.TODO(), bytes.NewBuffer(s.uploadedContent), s.casResource)
	require.NoError(s.T(), err)

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			g, err := s.validBackend.Describe(context.Background(), tc.digest)
			if tc.wantErr {
				assert.ErrorContains(err, tc.errMsg)
			} else {
				assert.NoError(err)
				assert.Equal(tc.want, g)
			}
		})
	}
}

func (s *testSuite) TestDownload() {
	testCases := []struct {
		name    string
		digest  string
		writer  *bytes.Buffer
		wantErr bool
		errMsg  string
		want    string
	}{
		{
			name:    "empty digest",
			digest:  "",
			wantErr: true,
			errMsg:  "digest is empty",
		},
		{
			name:   "get content",
			digest: s.casResource.Digest,
			writer: bytes.NewBuffer(nil),
			want:   "hello world",
		},
	}

	assert := assert.New(s.T())
	// Upload a demo image
	err := s.validBackend.Upload(context.TODO(), bytes.NewBuffer(s.uploadedContent), s.casResource)
	require.NoError(s.T(), err)

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			err := s.validBackend.Download(context.Background(), tc.writer, tc.digest)
			if tc.wantErr {
				assert.ErrorContains(err, tc.errMsg)
			} else {
				assert.NoError(err)
				assert.Equal(tc.want, tc.writer.String())
			}
		})
	}
}

func (s *testSuite) TestCraftImage() {
	testCases := []struct {
		name     string
		filename string
		digest   string
		content  []byte
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "empty content",
			filename: s.casResource.FileName,
			digest:   s.casResource.Digest,
			content:  nil,
			wantErr:  true,
			errMsg:   "content is empty",
		},
		{
			name:    "missing filename",
			digest:  s.casResource.Digest,
			content: s.uploadedContent,
			wantErr: true,
			errMsg:  "metadata is not valid",
		},
		{
			name:     "missing digest",
			filename: s.casResource.FileName,
			content:  s.uploadedContent,
			wantErr:  true,
			errMsg:   "metadata is not valid",
		},
		{
			name:     "valid image",
			filename: s.casResource.FileName,
			digest:   s.casResource.Digest,
			content:  s.uploadedContent,
			wantErr:  false,
		},
	}

	assert := assert.New(s.T())
	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			img, err := craftImage(tc.content, &pb.CASResource{FileName: tc.filename, Digest: tc.digest})
			if tc.wantErr {
				assert.ErrorContains(err, tc.errMsg)
			} else {
				assert.NoError(err)

				// Check the image content
				mt, err := img.MediaType()
				assert.NoError(err)
				assert.True(mt.IsImage())

				// Annotations
				m, err := img.Manifest()
				assert.NoError(err)

				v, ok := m.Annotations[ocispec.AnnotationAuthors]
				assert.True(ok)
				assert.Equal(v, "chainloop.dev")
				v, ok = m.Annotations[ocispec.AnnotationTitle]
				assert.True(ok)
				assert.Equal(v, tc.filename)

				// Layer
				layers, err := img.Layers()
				assert.NoError(err)
				assert.Len(layers, 1)
				wantD, err := layers[0].Digest()
				assert.NoError(err)
				assert.Equal(wantD.Hex, tc.digest)
			}
		})
	}
}

func (s *testSuite) TestCheckWritePermissions() {
	t := s.T()
	assert := assert.New(t)
	expectedRepo := "chainloop-test"
	tc := []struct {
		status  int
		wantErr bool
	}{
		{http.StatusCreated, false},
		{http.StatusAccepted, false},
		{http.StatusForbidden, true},
		{http.StatusBadRequest, true},
	}

	for _, c := range tc {
		t.Run(fmt.Sprintf("status=%d", c.status), func(t *testing.T) {
			initiatePath := fmt.Sprintf("/v2/%s/blobs/uploads/", expectedRepo)
			somewhereElse := fmt.Sprintf("/v2/%s/blobs/uploads/somewhere/else", expectedRepo)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/":
					w.WriteHeader(http.StatusOK)
				case initiatePath:
					assert.Equal(http.MethodPost, r.Method)
					w.Header().Set("Location", "somewhere/else")
					http.Error(w, "", c.status)
				case somewhereElse:
					assert.Equal(http.MethodDelete, r.Method)
				default:
					require.Fail(t, "Unexpected path: %v", r.URL.Path)
				}
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			assert.NoError(err)

			b := &Backend{repo: u.Host, keychain: authn.DefaultKeychain}

			got := b.CheckWritePermissions(context.Background())
			if c.wantErr {
				assert.Error(got)
			} else {
				assert.NoError(got)
			}
		})
	}
}

type testSuite struct {
	suite.Suite
	validBackend    *Backend
	server          *httptest.Server
	uploadedContent []byte
	casResource     *pb.CASResource
}

// Before the suite
func (s *testSuite) SetupSuite() {
	s.uploadedContent = []byte("hello world")
	s.casResource = &pb.CASResource{Digest: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", FileName: "test.txt", Size: 11}
}

// Before each test
func (s *testSuite) SetupTest() {
	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))

	u, err := url.Parse(server.URL)
	require.NoError(s.T(), err)

	b, err := NewBackend(u.Host, &RegistryOptions{})
	require.NoError(s.T(), err)

	s.validBackend = b
	s.server = server
}

func (s *testSuite) TearDownTest() {
	s.server.Close()
}

func TestOCIBackend(t *testing.T) {
	suite.Run(t, new(testSuite))
}
