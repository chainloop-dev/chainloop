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

//nolint:dupl
package materials_test

import (
	"context"
	"strings"
	"testing"

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	attestationApi "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	mUploader "github.com/chainloop-dev/chainloop/internal/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCSAFCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path VEX",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_CSAF_VEX,
			},
		},
		{
			name: "happy path Informational Advisory",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_CSAF_INFORMATIONAL_ADVISORY,
			},
		},
		{
			name: "happy path Security Advisory",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_CSAF_SECURITY_ADVISORY,
			},
		},
		{
			name: "happy path Security Incident Response",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_CSAF_SECURITY_INCIDENT_RESPONSE,
			},
		},
		{
			name: "wrong type",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			switch tc.input.Type {
			case contractAPI.CraftingSchema_Material_CSAF_VEX:
				_, err = materials.NewCSAFVEXCrafter(tc.input, nil, nil)
			case contractAPI.CraftingSchema_Material_CSAF_INFORMATIONAL_ADVISORY:
				_, err = materials.NewCSAFInformationalAdvisoryCrafter(tc.input, nil, nil)
			case contractAPI.CraftingSchema_Material_CSAF_SECURITY_ADVISORY:
				_, err = materials.NewCSAFSecurityAdvisoryCrafter(tc.input, nil, nil)
			case contractAPI.CraftingSchema_Material_CSAF_SECURITY_INCIDENT_RESPONSE:
				_, err = materials.NewCSAFSecurityIncidentResponseCrafter(tc.input, nil, nil)
			default:
				// For example VEX crafter so, we fail if the material is not ok
				_, err = materials.NewCSAFVEXCrafter(tc.input, nil, nil)
			}

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestCSAFCraft(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
		digest   string
	}{
		{
			name:     "non-expected json file",
			filePath: "./testdata/sbom.cyclonedx.json",
			wantErr:  "unexpected material type",
		},
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.json",
			wantErr:  "unexpected material type",
		},
		{
			name:     "invalid artifact type",
			filePath: "./testdata/simple.txt",
			wantErr:  "unexpected material type",
		},
		{
			name:     "valid artifact type",
			filePath: "./testdata/csaf_vex_v0.2.0.json",
			digest:   "sha256:d38f293e130fbb01d72b1df0b53a9eb1f0b50dd2053665db881d56ed9f4107c2",
		},
		{
			name:     "2.0 security advisory",
			filePath: "./testdata/csaf_security_advisory.json",
			digest:   "sha256:f1b3429e94e2e3b470402fa436b89f432d5209c6c8a12164cfccc90ec2637324",
		},
		{
			name:     "2.0 informational advisory",
			filePath: "./testdata/csaf_informational_advisory.json",
			digest:   "sha256:015fc9b32648fec3f5b719ef52161aef130eba164b187289ea65d3fa4d7e2f2a",
		},
		{
			name:     "2.0 security incident response",
			filePath: "./testdata/csaf_security_incident_response.json",
			digest:   "sha256:01674c1f6fbea901989369f73c6ba66a5f2c39cc57b542bb9cfbfddcc4106a2e",
		},
	}

	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_CSAF_VEX,
	}
	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock uploader
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("UploadFile", context.TODO(), tc.filePath).
					Return(&casclient.UpDownStatus{
						Digest:   "deadbeef",
						Filename: "vex.json",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}

			var crafter *materials.CSAFCrafter
			var err error
			switch schema.Type {
			case contractAPI.CraftingSchema_Material_CSAF_VEX:
				crafter, err = materials.NewCSAFVEXCrafter(schema, backend, &l)
			case contractAPI.CraftingSchema_Material_CSAF_INFORMATIONAL_ADVISORY:
				crafter, err = materials.NewCSAFInformationalAdvisoryCrafter(schema, backend, &l)
			case contractAPI.CraftingSchema_Material_CSAF_SECURITY_ADVISORY:
				crafter, err = materials.NewCSAFSecurityAdvisoryCrafter(schema, backend, &l)
			case contractAPI.CraftingSchema_Material_CSAF_SECURITY_INCIDENT_RESPONSE:
				crafter, err = materials.NewCSAFSecurityIncidentResponseCrafter(schema, backend, &l)
			}

			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			assert.Equal(contractAPI.CraftingSchema_Material_CSAF_VEX.String(), got.MaterialType.String())
			assert.True(got.UploadedToCas)

			// // The result includes the digest reference
			assert.Equal(&attestationApi.Attestation_Material_Artifact{
				Id: "test", Digest: tc.digest, Name: strings.Split(tc.filePath, "/")[2],
			}, got.GetArtifact())
		})
	}
}
