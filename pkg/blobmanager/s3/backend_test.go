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

package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/docker/go-connections/nat"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func (s *testSuite) TestHexSha256ToBinaryB64() {
	testCases := []struct {
		name     string
		hexSha   string
		expected string
	}{
		{
			name:     "valid sha",
			hexSha:   "aabbccddeeff",
			expected: "qrvM3e7/",
		},
		{
			name:     "invalid sha",
			hexSha:   "aabbccddeeffgg",
			expected: "",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			actual := hexSha256ToBinaryB64(tc.hexSha)
			s.Equal(tc.expected, actual)
		})
	}
}

func (s *testSuite) TestResourceName() {
	testCases := []struct {
		name     string
		sha      string
		expected string
	}{
		{
			name:     "valid sha",
			sha:      "aabbccddeeff",
			expected: "sha256:aabbccddeeff",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			actual := resourceName(tc.sha)
			s.Equal(tc.expected, actual)
		})
	}
}

func (s *testSuite) TestWritePermissions() {
	s.T().Run("invalid credentials", func(t *testing.T) {
		s.Error(s.invalidBackend.CheckWritePermissions(context.Background()))
	})

	s.T().Run("valid credentials", func(t *testing.T) {
		s.NoError(s.backend.CheckWritePermissions(context.Background()))
	})
}

func (s *testSuite) TestExists() {
	s.T().Run("doesn't exist", func(t *testing.T) {
		found, err := s.backend.Exists(context.Background(), "aabbccddeeff")
		s.NoError(err)
		s.False(found)
	})

	s.T().Run("found", func(t *testing.T) {
		found, err := s.backend.Exists(context.Background(), s.ownedObjectDigest)
		s.NoError(err)
		s.True(found)
	})

	s.T().Run("exists but not uploaded by chainloop", func(t *testing.T) {
		found, err := s.backend.Exists(context.Background(), s.externalObjectDigest)
		s.ErrorContains(err, "not uploaded by Chainloop")
		s.False(found)
	})
}

func (s *testSuite) TestDescribe() {
	s.T().Run("doesn't exist", func(t *testing.T) {
		artifact, err := s.backend.Describe(context.Background(), "aabbccddeeff")
		s.Error(err)
		s.True(backend.IsNotFound(err))
		s.Nil(artifact)
	})

	s.T().Run("found", func(t *testing.T) {
		artifact, err := s.backend.Describe(context.Background(), s.ownedObjectDigest)
		s.NoError(err)
		s.Equal("test.txt", artifact.FileName)
		s.Equal(s.ownedObjectDigest, artifact.Digest)
		s.Equal(int64(4), artifact.Size)
	})
}
func (s *testSuite) TestChecksumVerificationEnabled() {
	testCases := []struct {
		name           string
		customEndpoint string
		expected       bool
	}{
		{
			name:           "no endpoint, a.k.a AWS",
			customEndpoint: "",
			expected:       true,
		},
		{
			name:           "custom endpoint, i.e minio",
			customEndpoint: s.minio.ConnectionString(s.T()),
			expected:       true,
		},
		{
			name:           "custom endpoint",
			customEndpoint: "https://123.r2.cloudflarestorage.com/bucket-name",
			expected:       false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			b := &Backend{customEndpoint: tc.customEndpoint}
			s.Equal(tc.expected, b.checksumVerificationEnabled())
		})
	}
}

func (s *testSuite) TestExtractLocationAndBucket() {
	type expected struct {
		endpoint string
		bucket   string
		err      string
	}

	testCases := []struct {
		name     string
		creds    *Credentials
		expected *expected
	}{
		{
			name: "no location",
			creds: &Credentials{
				BucketName: "bucket",
			},
			expected: &expected{
				bucket: "bucket",
			},
		},
		{
			name: "location is a bucket name",
			creds: &Credentials{
				Location: "bucket",
			},
			expected: &expected{
				bucket: "bucket",
			},
		},
		{
			name: "location is a URL",
			creds: &Credentials{
				Location: "https://custom-domain/bucket",
			},
			expected: &expected{
				endpoint: "https://custom-domain",
				bucket:   "bucket",
			},
		},
		{
			name: "invalid URL",
			creds: &Credentials{
				Location: "https://custom-domain",
			},
			expected: &expected{
				err: "doesn't contain a bucket name",
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			endpoint, bucket, err := extractLocationAndBucket(tc.creds)
			if tc.expected.err != "" {
				s.ErrorContains(err, tc.expected.err)
			} else {
				s.NoError(err)
				s.Equal(tc.expected.endpoint, endpoint)
				s.Equal(tc.expected.bucket, bucket)
			}
		})
	}
}

func (s *testSuite) TestDownload() {
	s.T().Run("exist but not uploaded by Chainloop", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		err := s.backend.Download(context.Background(), buf, s.externalObjectDigest)
		s.ErrorContains(err, "asset not uploaded by Chainloop")
		s.Empty(buf)
	})

	s.T().Run("doesn't exist", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		err := s.backend.Download(context.Background(), buf, "deadbeef")
		s.ErrorContains(err, "artifact not found")
		s.Empty(buf)
	})

	s.T().Run("exists", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		err := s.backend.Download(context.Background(), buf, s.ownedObjectDigest)
		s.NoError(err)
		s.Equal("test", buf.String())
	})

	s.T().Run("it's been tampered", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		err := s.backend.Download(context.Background(), buf, s.tamperedObjectDigest)
		s.ErrorContains(err, "failed to validate integrity of object")
	})
}

type testSuite struct {
	suite.Suite
	minio                   *minioInstance
	backend, invalidBackend *Backend
	ownedObjectDigest       string
	externalObjectDigest    string
	tamperedObjectDigest    string
}

func TestS3Backend(t *testing.T) {
	suite.Run(t, new(testSuite))
}

func (s *testSuite) SetupSuite() {
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		s.T().Skip()
	}
}

// Run before each test
const testBucket = "test-bucket"

func (s *testSuite) SetupTest() {
	s.minio = newMinioInstance(s.T())
	location := fmt.Sprintf("http://%s/%s", s.minio.ConnectionString(s.T()), testBucket)

	// Create backend
	backend, err := NewBackend(&Credentials{
		AccessKeyID:     "root",
		SecretAccessKey: "test-password",
		Region:          "us-east-1",
		Location:        location,
	})
	require.NoError(s.T(), err)
	s.backend = backend

	invalidBackend, err := NewBackend(&Credentials{
		AccessKeyID:     "root",
		SecretAccessKey: "wrong-password",
		Region:          "us-east-1",
		BucketName:      testBucket,
		Location:        location,
	})

	require.NoError(s.T(), err)
	s.invalidBackend = invalidBackend

	// create bucket
	minioClient, err := minio.New(s.minio.ConnectionString(s.T()), &minio.Options{
		Creds: credentials.NewStaticV4("root", "test-password", ""), Secure: false,
	})
	require.NoError(s.T(), err)
	require.NoError(s.T(), minioClient.MakeBucket(context.TODO(), testBucket, minio.MakeBucketOptions{}))

	// upload a valid artifact
	buf := bytes.NewBuffer([]byte("test"))
	s.ownedObjectDigest = fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
	// calculate sha256 of the content in the buffer
	err = s.backend.Upload(context.Background(), buf, &pb.CASResource{Digest: s.ownedObjectDigest, FileName: "test.txt"})
	require.NoError(s.T(), err)

	// Copy an existing object but reference it from somewhere else
	s.tamperedObjectDigest = "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"
	_, err = minioClient.CopyObject(context.Background(), minio.CopyDestOptions{
		Bucket: testBucket, Object: fmt.Sprintf("sha256:%s", s.tamperedObjectDigest),
	}, minio.CopySrcOptions{
		Bucket: testBucket, Object: fmt.Sprintf("sha256:%s", s.ownedObjectDigest),
	})
	require.NoError(s.T(), err)

	// upload another one but by the client directly
	reader := bytes.NewReader([]byte("hello world"))
	s.externalObjectDigest = "external-deadbeef"
	_, err = minioClient.PutObject(context.Background(), testBucket, fmt.Sprintf("sha256:%s", s.externalObjectDigest), reader, reader.Size(),
		minio.PutObjectOptions{})
	require.NoError(s.T(), err)
}

func (s *testSuite) TearDownTest() {
	if s.minio == nil {
		return
	}

	testcontainers.CleanupContainer(s.T(), s.minio.instance, testcontainers.StopTimeout(time.Minute))
}

func newMinioInstance(t *testing.T) *minioInstance {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	port, err := nat.NewPort("", "9000")
	require.NoError(t, err)

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:RELEASE.2023-09-04T19-57-37Z",
		ExposedPorts: []string{port.Port()},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "root",
			"MINIO_ROOT_PASSWORD": "test-password",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForListeningPort(port).WithStartupTimeout(5 * time.Minute),
	}

	instance, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	return &minioInstance{instance}
}

func (c *minioInstance) ConnectionString(t *testing.T) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	p, err := c.instance.MappedPort(ctx, "9000")
	assert.NoError(t, err)

	return fmt.Sprintf("0.0.0.0:%d", p.Int())
}

type minioInstance struct {
	instance testcontainers.Container
}
