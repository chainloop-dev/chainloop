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

package guac

import (
	"context"
	"io"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *testSuite) TestLoadBucket() {
	testCases := []struct {
		name   string
		bucket string
		errMsg string
	}{
		{"existing bucket", "existing-bucket", ""},
		{"non-existing bucket", "non-existing-bucket", "the bucket non-existing-bucket does not exist"},
		{"no bucket provided", "", "no bucket provided"},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			_, err := loadBucket(context.Background(), s.client, tc.bucket)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func (s *testSuite) TestUpload() {
	var content = []byte("test")
	const fileName = "sbom.json"
	metadata := &sdk.ChainloopMetadata{
		WorkflowID:      "wid",
		WorkflowRunID:   "wid",
		WorkflowName:    "name",
		WorkflowProject: "project",
	}

	// Perform the upload
	bucket, err := loadBucket(context.Background(), s.client, s.bucket)
	require.NoError(s.T(), err)

	l := log.NewStdLogger(io.Discard)
	err = uploadToBucket(context.Background(), bucket, fileName, content, metadata, log.NewHelper(l))
	require.NoError(s.T(), err)

	// Check the uploaded file
	got := bucket.Object("sbom.json")
	attrs, err := got.Attrs(context.Background())
	require.NoError(s.T(), err)

	// Content
	assert.Equal(s.T(), int64(len(content)), attrs.Size)
	// Metadata
	assert.Equal(s.T(), map[string]string{
		"workflowID":      "wid",
		"workflowRunID":   "wid",
		"workflowName":    "name",
		"workflowProject": "project",
		"author":          "chainloop",
		"filename":        "sbom.json",
	}, attrs.Metadata)

	// Content type override
	assert.Equal(s.T(), "application/json", attrs.ContentType)
}

func (s *testSuite) TestUniqueFilename() {
	testCases := []struct {
		path     string
		filename string
		expected string
	}{
		{"", "sbom.json", "sbom-deadbeef.json"},
		{"", "attestation.json", "attestation-deadbeef.json"},
		{"", "sbom-cyclone-dx-123.xml", "sbom-cyclone-dx-123-deadbeef.xml"},
		{"path", "attestation.json", "path/attestation-deadbeef.json"},
	}

	for _, tc := range testCases {
		got := uniqueFilename(tc.path, tc.filename, "deadbeef")
		assert.Equal(s.T(), tc.expected, got)
	}
}

type testSuite struct {
	suite.Suite
	client *storage.Client
	bucket string
}

func (s *testSuite) SetupTest() {
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "existing-bucket"},
		},
	})
	defer server.Stop()

	s.client = server.Client()
	s.bucket = "existing-bucket"
}

// Run all the tests
func TestGuacIntegration(t *testing.T) {
	suite.Run(t, new(testSuite))
}
