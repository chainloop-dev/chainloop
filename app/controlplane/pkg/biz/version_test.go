//
// Copyright 2024 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package biz_test

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/stretchr/testify/suite"
)

type versionTestSuite struct {
	suite.Suite
}

func (s *versionTestSuite) TestValidateVersion() {
	testCases := []struct {
		name      string
		version   string
		wantError bool
	}{
		{
			name:      "valid version with numbers only",
			version:   "123",
			wantError: false,
		},
		{
			name:      "valid version with letters only",
			version:   "abc",
			wantError: false,
		},
		{
			name:      "valid version with dots",
			version:   "1.2.3",
			wantError: false,
		},
		{
			name:      "valid version with hyphens",
			version:   "release-1.2.3",
			wantError: false,
		},
		{
			name:      "valid version with underscore",
			version:   "release_1.2.3",
			wantError: false,
		},
		{
			name:      "valid complex version",
			version:   "v1.2.3-alpha.1",
			wantError: false,
		},
		{
			name:      "valid version with build metadata",
			version:   "1.0.0+001",
			wantError: false,
		},
		{
			name:      "valid complex version with build metadata",
			version:   "v1.2.3-alpha.1+build.123",
			wantError: false,
		},
		{
			name:      "valid date based version",
			version:   "20230615",
			wantError: false,
		},
		{
			name:      "valid date based version with dots",
			version:   "2023.06.15",
			wantError: false,
		},
		{
			name:      "valid date based version with prefix",
			version:   "release-20230615",
			wantError: false,
		},
		{
			name:      "valid date based version with underscore",
			version:   "release_20230615",
			wantError: false,
		},
		{
			name:      "invalid version with spaces",
			version:   "version 1.0",
			wantError: true,
		},
		{
			name:      "invalid version with special chars",
			version:   "v1.0@beta",
			wantError: true,
		},
		{
			name:      "empty version",
			version:   "",
			wantError: true,
		},
		{
			name:      "with spaces",
			version:   "v1 prerelease",
			wantError: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			err := biz.ValidateVersion(tc.version)
			if tc.wantError {
				s.Error(err)
				s.True(biz.IsErrValidation(err))
			} else {
				s.NoError(err)
			}
		})
	}
}
func TestVersionSuite(t *testing.T) {
	suite.Run(t, new(versionTestSuite))
}
