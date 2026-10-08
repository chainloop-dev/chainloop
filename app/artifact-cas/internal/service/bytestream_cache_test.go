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
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/mocks"
	"github.com/chainloop-dev/chainloop/pkg/cache"
	"github.com/chainloop-dev/chainloop/pkg/cache/casexistence"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

const otherOrgID = "2b7d0d6e-7d3f-4a39-9a43-7c1d2f0e9b11"

// write uploads data as resource in a single chunk
func (s *bytestreamSuite) write(ctx context.Context, resource *v1.CASResource, data []byte) (*bytestream.WriteResponse, error) {
	stream, err := s.client.Write(ctx)
	s.Require().NoError(err)
	s.Require().NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), resource),
		Data:         data,
	}))

	return stream.CloseAndRecv()
}

func uploaderCtx(kv ...string) context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs(append([]string{"role", string(casJWT.Uploader)}, kv...)...))
}

// restart replaces the server with one that also has opts
func (s *bytestreamSuite) restart(opts ...NewOpt) {
	s.TearDownTest()
	s.setup(opts...)
}

// withExistenceCache restarts the server with the existence cache only, so a
// cached hit can be told apart from a reused backend client.
func (s *bytestreamSuite) withExistenceCache() {
	c, err := cache.New[casexistence.Entry](cache.WithTTL(time.Hour))
	s.Require().NoError(err)
	s.restart(WithExistenceCache(c))
}

// withClientCache restarts the server with the backend client cache only
func (s *bytestreamSuite) withClientCache() {
	s.restart(WithBackendClientCache(time.Hour))
}

// R-001, R-006: a second upload of a blob that the backend reported as present
// makes no secrets manager or backend call, and its event keeps the size.
func (s *bytestreamSuite) TestExistenceCacheHitSkipsBackend() {
	s.withExistenceCache()
	s.ociBackend.On("Exists", mock.Anything, s.resource.Digest).Return(true, nil).Once()
	s.ociBackend.On("Describe", mock.Anything, s.resource.Digest).Return(&v1.CASResource{
		FileName: s.resource.FileName, Digest: s.resource.Digest, Size: 1024,
	}, nil).Once()

	for range 2 {
		got, err := s.write(s.upCtx, s.resource, nil)
		s.Require().NoError(err)
		s.Equal(int64(0), got.CommittedSize)
	}

	s.ociProvider.AssertNumberOfCalls(s.T(), "FromCredentials", 1)
	s.ociBackend.AssertNumberOfCalls(s.T(), "Exists", 1)
	s.ociBackend.AssertNumberOfCalls(s.T(), "Describe", 1)

	s.Require().Len(s.audit.published, 2)
	for _, e := range s.audit.published {
		info := decodeArtifactEvent(s.T(), e)
		s.True(info.Skipped)
		s.Equal(int64(1024), info.SizeBytes)
		s.Equal(s.resource.FileName, info.FileName)
	}
}

// R-002: a successful upload is cached with its committed size.
func (s *bytestreamSuite) TestExistenceCacheUploadSuccessIsCached() {
	s.withExistenceCache()
	data := []byte("hello world")
	resource := resourceWithDigest(data, "skynet.exe")
	s.ociBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil).Once()
	s.ociBackend.On("Upload", mock.Anything, mock.Anything, resource).Return(nil).Once()

	got, err := s.write(s.upCtx, resource, data)
	s.Require().NoError(err)
	s.Equal(int64(len(data)), got.CommittedSize)

	got, err = s.write(s.upCtx, resource, data)
	s.Require().NoError(err)
	s.Equal(int64(0), got.CommittedSize)

	s.ociProvider.AssertNumberOfCalls(s.T(), "FromCredentials", 1)
	s.Require().Len(s.audit.published, 2)
	info := decodeArtifactEvent(s.T(), s.audit.published[1])
	s.True(info.Skipped)
	s.Equal(int64(len(data)), info.SizeBytes)
}

// R-002: a "not found" answer, or a found blob without a known size, is not cached.
func (s *bytestreamSuite) TestExistenceCacheNotCached() {
	data := []byte("hello world")
	resource := resourceWithDigest(data, "skynet.exe")

	testCases := []struct {
		name  string
		setup func()
		code  codes.Code
	}{
		{
			name: "not found and failed upload",
			setup: func() {
				s.ociBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil)
				s.ociBackend.On("Upload", mock.Anything, mock.Anything, resource).Return(errors.New("boom"))
			},
			code: codes.Internal,
		},
		{
			name: "found but describe fails",
			setup: func() {
				s.ociBackend.On("Exists", mock.Anything, resource.Digest).Return(true, nil)
				s.ociBackend.On("Describe", mock.Anything, resource.Digest).Return(nil, errors.New("boom"))
			},
			code: codes.OK,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.withExistenceCache()
			tc.setup()
			for range 2 {
				_, err := s.write(s.upCtx, resource, data)
				if tc.code == codes.OK {
					s.Require().NoError(err)
				} else {
					assertGRPCError(s.T(), err, tc.code, "")
				}
			}
			s.ociBackend.AssertNumberOfCalls(s.T(), "Exists", 2)
		})
	}
}

// R-003: an entry is only visible to the same organization and backend.
func (s *bytestreamSuite) TestExistenceCacheTenantNamespace() {
	testCases := []struct {
		name      string
		secondCtx context.Context
		// backend the second upload must reach
		second func() *mockedBackend
	}{
		{
			name:      "different organization on shared storage",
			secondCtx: uploaderCtx("org-id", otherOrgID),
			second:    func() *mockedBackend { return &mockedBackend{s.ociBackend, 2} },
		},
		{
			name:      "same organization, different backend",
			secondCtx: uploaderCtx("backend-streaming", "true"),
			second:    func() *mockedBackend { return &mockedBackend{s.streamingBackend, 1} },
		},
		{
			name:      "organization missing from the token",
			secondCtx: uploaderCtx("org-id", ""),
			second:    func() *mockedBackend { return &mockedBackend{s.ociBackend, 2} },
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.withExistenceCache()
			describe := &v1.CASResource{Digest: s.resource.Digest, Size: 1}
			for _, b := range []*mockedBackend{{s.ociBackend, 0}, {s.streamingBackend, 0}} {
				b.On("Exists", mock.Anything, s.resource.Digest).Maybe().Return(true, nil)
				b.On("Describe", mock.Anything, s.resource.Digest).Maybe().Return(describe, nil)
			}

			_, err := s.write(s.upCtx, s.resource, nil)
			s.Require().NoError(err)
			_, err = s.write(tc.secondCtx, s.resource, nil)
			s.Require().NoError(err)

			want := tc.second()
			want.AssertNumberOfCalls(s.T(), "Exists", want.calls)
		})
	}
}

// R-008: a failing cache is a miss and never fails the upload.
func (s *bytestreamSuite) TestExistenceCacheFailureIsMiss() {
	s.restart(WithExistenceCache(failingCache{}))

	data := []byte("hello world")
	resource := resourceWithDigest(data, "skynet.exe")
	s.ociBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil)
	s.ociBackend.On("Upload", mock.Anything, mock.Anything, resource).Return(nil)

	for range 2 {
		got, err := s.write(s.upCtx, resource, data)
		s.Require().NoError(err)
		s.Equal(int64(len(data)), got.CommittedSize)
	}
	s.ociBackend.AssertNumberOfCalls(s.T(), "Upload", 2)
}

// R-007: a cache miss on the existence check reuses the loaded backend client.
func (s *bytestreamSuite) TestClientCacheReusesClient() {
	s.withClientCache()
	s.ociBackend.On("Exists", mock.Anything, mock.Anything).Return(true, nil)

	for _, d := range []string{"digest-a", "digest-b"} {
		_, err := s.write(uploaderCtx("source-internal", "true"), &v1.CASResource{Digest: d}, nil)
		s.Require().NoError(err)
	}
	s.ociProvider.AssertNumberOfCalls(s.T(), "FromCredentials", 1)

	// a client is never shared between organizations
	_, err := s.write(uploaderCtx("source-internal", "true", "org-id", otherOrgID), s.resource, nil)
	s.Require().NoError(err)
	s.ociProvider.AssertNumberOfCalls(s.T(), "FromCredentials", 2)
}

// D-009: an error from a reused client drops it and loads it again one time.
func (s *bytestreamSuite) TestClientCacheReloadOnError() {
	s.withClientCache()
	ctx := uploaderCtx("source-internal", "true")
	s.ociBackend.On("Exists", mock.Anything, mock.Anything).Return(true, nil).Once()
	_, err := s.write(ctx, s.resource, nil)
	s.Require().NoError(err)

	s.ociBackend.On("Exists", mock.Anything, mock.Anything).Return(false, errors.New("expired credentials")).Once()
	s.ociBackend.On("Exists", mock.Anything, mock.Anything).Return(true, nil).Once()
	_, err = s.write(ctx, s.resource, nil)
	s.Require().NoError(err)
	s.ociProvider.AssertNumberOfCalls(s.T(), "FromCredentials", 2)
}

// a fresh client that fails is not loaded again
func (s *bytestreamSuite) TestClientCacheNoReloadOfFreshClient() {
	s.withClientCache()
	s.ociBackend.On("Exists", mock.Anything, mock.Anything).Return(false, errors.New("boom"))

	_, err := s.write(s.upCtx, s.resource, nil)
	assertGRPCError(s.T(), err, codes.Internal, "")
	s.ociProvider.AssertNumberOfCalls(s.T(), "FromCredentials", 1)
	s.ociBackend.AssertNumberOfCalls(s.T(), "Exists", 1)
}

type mockedBackend struct {
	*mocks.UploaderDownloader
	calls int
}

type failingCache struct{}

var errCacheDown = errors.New("cache is down")

func (failingCache) Get(context.Context, string) (casexistence.Entry, bool, error) {
	return casexistence.Entry{}, false, errCacheDown
}
func (failingCache) Set(context.Context, string, casexistence.Entry) error { return errCacheDown }
func (failingCache) Delete(context.Context, string) error                  { return errCacheDown }
func (failingCache) Purge(context.Context) error                           { return errCacheDown }

func TestExistenceCacheKey(t *testing.T) {
	base := casexistence.Key("org", "OCI", "secret", "digest")
	require.NotContains(t, base, "secret", "the secret reference must be hashed")
	for _, other := range []string{
		casexistence.Key("org2", "OCI", "secret", "digest"),
		casexistence.Key("org", "S3", "secret", "digest"),
		casexistence.Key("org", "OCI", "secret2", "digest"),
		casexistence.Key("org", "OCI", "secret", "digest2"),
	} {
		require.NotEqual(t, base, other)
	}
}
