//
// Copyright 2023-2025 The Chainloop Authors.
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
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	repoM "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	v1 "github.com/google/go-containerregistry/pkg/v1"
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
		CASBackend:    &biz.CASBackend{ID: validUUID},
		WorkflowRunID: validUUID,
		OrgID:         validUUID,
		Public:        false,
	}

	// Mock successful repo call
	s.repo.On("Create", mock.Anything, validDigest, validUUID, biz.ToPtr(validUUID)).Return(want, nil).Maybe()

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
					Digest: "sha256:63f811807585a7359882fc4e28bc8e08555d9743aa07a2965217b30ef2ba14a5",
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
					Digest: "sha256:b447f27683a88b55d529744d56c83c42fbe7d05692efaa6e5eddfadec392f812",
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

			h, _, err := v1.SHA256(bytes.NewReader(attJSON))
			require.NoError(s.T(), err)

			got, err := s.useCase.LookupDigestsInAttestation(envelope, h)
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
	s.useCase = biz.NewCASMappingUseCase(s.repo, nil, nil)
}

func TestCASMapping(t *testing.T) {
	suite.Run(t, new(casMappingSuite))
}
