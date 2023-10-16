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

package s3

import (
	"testing"

	"github.com/stretchr/testify/suite"
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

type testSuite struct {
	suite.Suite
}

func TestS3Backend(t *testing.T) {
	suite.Run(t, new(testSuite))
}
