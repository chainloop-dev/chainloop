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
	"time"

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
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{"nil config", nil, "nil config"},
		{"missing role arn", &Config{Region: "us-east-1"}, "base_role_arn is required"},
		{"malformed role arn", &Config{BaseRoleARN: "not-an-arn", Region: "us-east-1"}, "not a valid IAM role ARN"},
		{"missing region", &Config{BaseRoleARN: "arn:aws:iam::1:role/r"}, "region is required"},
		{"happy", &Config{BaseRoleARN: "arn:aws:iam::1:role/r", Region: "us-east-1"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestCredentials_Validate(t *testing.T) {
	t.Parallel()
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

func TestValidateAndExtractCredentials(t *testing.T) {
	t.Parallel()

	good := validCreds()
	goodJSON, _ := json.Marshal(good)

	// Same content but mismatched location passed alongside.
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
			p := &BackendProvider{cfg: &Config{
				BaseRoleARN: "arn:aws:iam::1:role/r", Region: "us-east-1",
			}}
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
		})
	}
}

func TestNewBackendProvider_NormalizesSessionDuration(t *testing.T) {
	cfg := &Config{
		BaseRoleARN: "arn:aws:iam::1:role/r",
		Region:      "us-east-1",
		// Intentionally zero — provider should fill the default.
	}
	p, err := NewBackendProvider(cfg, stubReader{})
	require.NoError(t, err)
	assert.Equal(t, ProviderID, p.ID())
	assert.Equal(t, DefaultSessionDuration, p.cfg.SessionDuration)

	custom := 5 * time.Minute
	cfg2 := &Config{BaseRoleARN: cfg.BaseRoleARN, Region: cfg.Region, SessionDuration: custom}
	p2, err := NewBackendProvider(cfg2, stubReader{})
	require.NoError(t, err)
	assert.Equal(t, custom, p2.cfg.SessionDuration)
}

func TestWithRequestingOrg_RoundTrip(t *testing.T) {
	// Empty by default.
	assert.Empty(t, requestingOrgFromContext(context.Background()))

	ctx := WithRequestingOrg(context.Background(), "org-abc")
	assert.Equal(t, "org-abc", requestingOrgFromContext(ctx))

	// Overwrite is allowed and uses the most recent value (mirrors
	// context.WithValue semantics — important so a middleware that sets
	// the org doesn't get silently overridden by a stale value further
	// down the stack).
	ctx = WithRequestingOrg(ctx, "org-xyz")
	assert.Equal(t, "org-xyz", requestingOrgFromContext(ctx))
}

func TestNewBackendProvider_FailsOnBadConfig(t *testing.T) {
	_, err := NewBackendProvider(&Config{Region: "us-east-1"}, stubReader{})
	assert.ErrorContains(t, err, "base_role_arn")

	_, err = NewBackendProvider(&Config{BaseRoleARN: "arn:aws:iam::1:role/r", Region: "us-east-1"}, nil)
	assert.ErrorContains(t, err, "credentials reader is required")
}

// Dev mode relaxes the base_role_arn requirement because nothing on the
// hot path will actually call sts:AssumeRole. Region is still required —
// the SDK config needs it to construct any S3 client at all.
func TestConfig_Validate_DevModeRelaxesBaseRoleARN(t *testing.T) {
	t.Parallel()

	// Without dev mode: empty base role rejected.
	err := (&Config{Region: "us-east-1"}).Validate()
	require.ErrorContains(t, err, "base_role_arn is required")

	// With dev mode: empty base role accepted.
	err = (&Config{Region: "us-east-1", DevModeUseAmbientCredentials: true}).Validate()
	require.NoError(t, err)

	// Region is still mandatory in dev mode.
	err = (&Config{DevModeUseAmbientCredentials: true}).Validate()
	require.ErrorContains(t, err, "region is required")
}

// stubReader is the minimal credentials.Reader implementation needed to
// exercise constructor wiring; the unit tests never invoke it.
type stubReader struct{}

func (stubReader) ReadCredentials(_ context.Context, _ string, _ any) error { return nil }
