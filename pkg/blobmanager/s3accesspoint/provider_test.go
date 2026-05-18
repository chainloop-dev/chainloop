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

package s3accesspoint

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validCreds is reused across the unit tests as a known-good baseline.
// Each test case clones and mutates it so we can express what's missing
// rather than what's present.
func validCreds() Credentials {
	return Credentials{
		AccessPointARN: "arn:aws:s3:us-east-1:123456789012:accesspoint/chainloop-org-abc",
		Region:         "us-east-1",
		BaseRoleARN:    "arn:aws:iam::123456789012:role/chainloop-cas-tenant",
	}
}

func TestCredentials_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Credentials)
		wantErr string
	}{
		{"happy", func(*Credentials) {}, ""},
		{
			name:    "missing arn",
			mutate:  func(c *Credentials) { c.AccessPointARN = "" },
			wantErr: "missing access_point_arn",
		},
		{
			name:    "not an AP arn",
			mutate:  func(c *Credentials) { c.AccessPointARN = "arn:aws:s3:::some-bucket" },
			wantErr: "not an S3 access point ARN",
		},
		{
			name:    "missing region",
			mutate:  func(c *Credentials) { c.Region = "" },
			wantErr: "missing region",
		},
		{
			name:    "missing base role arn",
			mutate:  func(c *Credentials) { c.BaseRoleARN = "" },
			wantErr: "missing base_role_arn",
		},
		{
			name:    "malformed base role arn",
			mutate:  func(c *Credentials) { c.BaseRoleARN = "not-an-arn" },
			wantErr: "not a valid IAM role ARN",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validCreds()
			tc.mutate(&c)
			err := c.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// In dev mode the base role requirement is relaxed because nothing on the
// hot path will actually call sts:AssumeRole. AccessPointARN and Region
// remain mandatory — the SDK needs the latter to construct any S3 client
// at all.
func TestCredentials_Validate_DevModeRelaxesBaseRoleARN(t *testing.T) {
	// Without dev mode: empty base role rejected.
	c := validCreds()
	c.BaseRoleARN = ""
	require.ErrorContains(t, c.Validate(), "missing base_role_arn")

	// With dev mode: empty base role accepted.
	t.Setenv(DevModeEnvVar, "true")
	require.NoError(t, c.Validate())

	// AccessPointARN is still mandatory in dev mode.
	c2 := validCreds()
	c2.AccessPointARN = ""
	require.ErrorContains(t, c2.Validate(), "missing access_point_arn")
}

func TestValidateAndExtractCredentials(t *testing.T) {
	t.Parallel()

	good := validCreds()
	goodJSON, _ := json.Marshal(good)
	wrongLocation := good.AccessPointARN + "-tampered"

	tests := []struct {
		name     string
		location string
		body     []byte
		wantErr  string
	}{
		{"valid no location", "", goodJSON, ""},
		{"valid matching location", good.AccessPointARN, goodJSON, ""},
		{"location mismatch", wrongLocation, goodJSON, "does not match access_point_arn"},
		{"malformed JSON", "", []byte("{not json"), "unmarshaling"},
		{"missing field", "", []byte(`{"AccessPointARN":""}`), "missing access_point_arn"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &BackendProvider{cReader: stubReader{}}
			out, err := p.ValidateAndExtractCredentials(tc.location, tc.body)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Nil(t, out)
				return
			}
			require.NoError(t, err)
			creds, ok := out.(*Credentials)
			require.True(t, ok, "expected *Credentials, got %T", out)
			assert.Equal(t, good.AccessPointARN, creds.AccessPointARN)
			assert.Equal(t, good.Region, creds.Region)
			assert.Equal(t, good.BaseRoleARN, creds.BaseRoleARN)
		})
	}
}

func TestNewBackendProvider(t *testing.T) {
	t.Parallel()

	p := NewBackendProvider(stubReader{})
	assert.Equal(t, ProviderID, p.ID())
}

// stubReader is the minimal credentials.Reader implementation needed to
// exercise constructor wiring; the unit tests never invoke it.
type stubReader struct{}

func (stubReader) ReadCredentials(_ context.Context, _ string, _ any) error { return nil }
