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

package dependencytrack_test

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/integration/dependencytrack"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	cmocks "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func (s *testSuite) TestAdd() {
	assert := assert.New(s.T())
	credsReader := cmocks.NewReaderWriter(s.T())
	ctx := context.Background()
	org, err := s.Organization.Create(ctx, "testing org")
	assert.NoError(err)

	i := dependencytrack.New(s.Integration, credsReader, nil, nil)

	credsReader.On("SaveCredentials", ctx, org.ID, mock.Anything).Return("secret-key", nil)

	got, err := i.Add(ctx, org.ID, "host", "key", true)

	assert.NoError(err)
	assert.Equal(dependencytrack.Kind, got.Kind)
	assert.Equal(true, got.Config.GetDependencyTrack().AllowAutoCreate)
	assert.Equal("host", got.Config.GetDependencyTrack().Domain)
	assert.Equal("secret-key", got.SecretName)
}

// Run the tests
func TestIntegration(t *testing.T) {
	suite.Run(t, new(testSuite))
}

// Utility struct to hold the test suite
type testSuite struct {
	testhelpers.UseCasesEachTestSuite
}
