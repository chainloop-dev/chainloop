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
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	repoM "github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/mocks"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *casMappingSuite) TestCreate() {
	validUUID := uuid.New()
	invalidUUID := "deadbeef"
	validDigest := "sha256:3b0f04c276be095e62f3ac03b9991913c37df1fcd44548e75069adce313aba4d"
	invalidDigest := "sha256:deadbeef"

	testCases := []struct {
		name                        string
		digest                      string
		casBackendID, workflowRunID string
		wantErr                     bool
	}{
		{
			name:          "valid",
			digest:        validDigest,
			casBackendID:  validUUID.String(),
			workflowRunID: validUUID.String(),
		},
		{
			name:          "invalid digest format",
			digest:        invalidDigest,
			casBackendID:  validUUID.String(),
			workflowRunID: validUUID.String(),
			wantErr:       true,
		},
		{
			name:          "invalid digest missing prefix",
			digest:        "3b0f04c276be095e62f3ac03b9991913c37df1fcd44548e75069adce313aba4d",
			casBackendID:  validUUID.String(),
			workflowRunID: validUUID.String(),
			wantErr:       true,
		},
		{
			name:          "invalid CASBackend",
			digest:        validDigest,
			casBackendID:  invalidUUID,
			workflowRunID: validUUID.String(),
			wantErr:       true,
		},
		{
			name:          "invalid WorkflowRunID",
			digest:        validDigest,
			casBackendID:  validUUID.String(),
			workflowRunID: invalidUUID,
			wantErr:       true,
		},
	}

	want := &biz.CASMapping{
		ID:            validUUID,
		Digest:        validDigest,
		CASBackendID:  validUUID,
		WorkflowRunID: validUUID,
		OrgID:         validUUID,
	}

	// Mock successful repo call
	s.repo.On("Create", mock.Anything, validDigest, validUUID, validUUID).Return(want, nil).Maybe()

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			got, err := s.useCase.Create(context.TODO(), tc.digest, tc.casBackendID, tc.workflowRunID)
			if tc.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(want, got)
			}
		})
	}
}

func (s *casMappingSuite) TestLookupDigestsInAttestation() {
	testCases := []struct {
		name    string
		attPath string
		want    []*biz.CASMappingLookupRef
		wantErr bool
	}{
		{
			name:    "full",
			attPath: "testdata/attestations/full.json",
			want: []*biz.CASMappingLookupRef{
				{
					Name:   "attestation",
					Digest: "sha256:1a077137aef7ca208b80c339769d0d7eecacc2850368e56e834cda1750ce413a",
				},
				{
					Name:   "skynet-sbom",
					Digest: "sha256:16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c",
				},
				{
					Name:   "skynet2-sbom",
					Digest: "sha256:16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c",
				},
			},
		},
		{
			name:    "no-materials",
			attPath: "testdata/attestations/empty.json",
			want: []*biz.CASMappingLookupRef{
				{
					Name:   "attestation",
					Digest: "sha256:1ad00b787214a1d09080a469390b15cdc3a751b89488da3776f432b4bbaa77d6",
				},
			},
		},
		{
			name:    "invalid-file",
			attPath: "testdata/attestations/invalid.json",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			attJSON, err := os.ReadFile(tc.attPath)
			require.NoError(s.T(), err)
			var envelope *dsse.Envelope
			require.NoError(s.T(), json.Unmarshal(attJSON, &envelope))

			got, err := s.useCase.LookupDigestsInAttestation(envelope)
			if tc.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(tc.want, got)
			}
		})
	}
}

type casMappingSuite struct {
	suite.Suite
	repo    *repoM.CASMappingRepo
	useCase *biz.CASMappingUseCase
}

func (s *casMappingSuite) SetupTest() {
	s.repo = repoM.NewCASMappingRepo(s.T())
	s.useCase = biz.NewCASMappingUseCase(s.repo, nil)
}

func TestCASMapping(t *testing.T) {
	suite.Run(t, new(casMappingSuite))
}
