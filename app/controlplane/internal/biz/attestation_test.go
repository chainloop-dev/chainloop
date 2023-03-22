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

package biz_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/mocks"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var runID = uuid.NewString()
var envelope = &dsse.Envelope{}

const expectedDigest = "f845058d865c3d4d491c9019f6afe9c543ad2cd11b31620cc512e341fb03d3d8"

func (s *attestationTestSuite) TestUploadToCAS() {
	ctx := context.Background()
	s.casClient.On(
		"Upload", ctx, "my-secret", mock.Anything,
		fmt.Sprintf("attestation-%s.json", runID), expectedDigest,
	).Return(nil)

	gotDigest, err := s.uc.UploadToCAS(ctx, envelope, "my-secret", runID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), expectedDigest, gotDigest)
}

func (s *attestationTestSuite) TestFetchFromStore() {
	want := &biz.Attestation{Envelope: &dsse.Envelope{}}

	ctx := context.Background()
	s.casClient.On("Download", ctx, "my-secret", mock.Anything, expectedDigest).Return(nil).Run(
		func(args mock.Arguments) {
			buf := args.Get(2).(io.Writer)
			err := json.NewEncoder(buf).Encode(want)
			require.NoError(s.T(), err)
		})

	got, err := s.uc.FetchFromStore(ctx, "my-secret", expectedDigest)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), want, got)
}

func TestAttestation(t *testing.T) {
	suite.Run(t, new(attestationTestSuite))
}

func (s *attestationTestSuite) SetupTest() {
	s.casClient = mocks.NewCASClient(s.T())
	s.uc = biz.NewAttestationUseCase(s.casClient, nil)
}

// Utility struct to hold the test suite
type attestationTestSuite struct {
	suite.Suite
	uc        *biz.AttestationUseCase
	casClient *mocks.CASClient
}
