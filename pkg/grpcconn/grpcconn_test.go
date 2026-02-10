//
// Copyright 2024-2026 The Chainloop Authors.
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

package grpcconn

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const caPath = "../../devel/devkeys/selfsigned/rootCA.crt"

func TestGetRequestMetadata(t *testing.T) {
	const wantOrg = "org-1"
	want := map[string]string{"authorization": "Bearer token", "Chainloop-Organization": wantOrg}
	auth := newTokenAuth("token", false, wantOrg)
	got, err := auth.GetRequestMetadata(context.TODO())
	assert.NoError(t, err)

	assert.Equal(t, got, want)
}

func TestRequireTransportSecurity(t *testing.T) {
	testCases := []struct {
		insecure bool
		want     bool
	}{
		{insecure: true, want: false},
		{insecure: false, want: true},
	}

	for _, tc := range testCases {
		auth := newTokenAuth("token", tc.insecure, "org-1")
		assert.Equal(t, tc.want, auth.RequireTransportSecurity())
	}
}

func TestIsFilePath(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.pem")
	err := os.WriteFile(tmpFile, []byte("test content"), 0600)
	require.NoError(t, err)

	testCases := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "absolute path",
			value: "/path/to/ca.pem",
			want:  true,
		},
		{
			name:  "relative path with ./",
			value: "./ca.pem",
			want:  true,
		},
		{
			name:  "relative path with ../",
			value: "../ca.pem",
			want:  true,
		},
		{
			name:  "home directory path",
			value: "~/ca.pem",
			want:  true,
		},
		{
			name:  "existing file",
			value: tmpFile,
			want:  true,
		},
		{
			name:  "PEM content with newlines",
			value: "-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\n-----END CERTIFICATE-----",
			want:  false,
		},
		{
			name:  "base64 encoded content",
			value: "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=",
			want:  false,
		},
		{
			name:  "non-existent file without path prefix",
			value: "nonexistent.pem",
			want:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsFilePath(tc.value)
			assert.Equal(t, tc.want, got, "IsFilePath(%q) = %v, want %v", tc.value, got, tc.want)
		})
	}
}

func TestAppendCAFromFile(t *testing.T) {
	// Check if the file exists, skip test if not
	if _, err := os.Stat(caPath); os.IsNotExist(err) {
		t.Skip("Test CA file not found, skipping test")
	}

	certsPool, err := x509.SystemCertPool()
	require.NoError(t, err)

	err = appendCAFromFile(caPath, certsPool)
	assert.NoError(t, err)
}

func TestAppendCAFromFile_NonExistent(t *testing.T) {
	certsPool, err := x509.SystemCertPool()
	require.NoError(t, err)

	err = appendCAFromFile("/nonexistent/ca.pem", certsPool)
	assert.Error(t, err)
}

func TestAppendCAFromContent_PEM(t *testing.T) {
	// Check if the file exists, skip test if not
	if _, err := os.Stat(caPath); os.IsNotExist(err) {
		t.Skip("Test CA file not found, skipping test")
	}

	// Read the PEM content
	pemContent, err := os.ReadFile(caPath)
	require.NoError(t, err)

	certsPool, err := x509.SystemCertPool()
	require.NoError(t, err)

	// Test with raw PEM content
	err = appendCAFromContent(string(pemContent), certsPool)
	assert.NoError(t, err)
}

func TestAppendCAFromContent_Base64(t *testing.T) {
	// Check if the file exists, skip test if not
	if _, err := os.Stat(caPath); os.IsNotExist(err) {
		t.Skip("Test CA file not found, skipping test")
	}

	// Read the PEM content and encode as base64
	pemContent, err := os.ReadFile(caPath)
	require.NoError(t, err)
	base64Content := base64.StdEncoding.EncodeToString(pemContent)

	certsPool, err := x509.SystemCertPool()
	require.NoError(t, err)

	// Test with base64-encoded content
	err = appendCAFromContent(base64Content, certsPool)
	assert.NoError(t, err)
}

func TestAppendCAFromContent_Invalid(t *testing.T) {
	certsPool, err := x509.SystemCertPool()
	require.NoError(t, err)

	// Test with invalid content
	err = appendCAFromContent("invalid certificate content", certsPool)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to append CA cert to pool")
}

func TestWithCAFile(t *testing.T) {
	opt := &newOptionalArg{}
	WithCAFile("/path/to/ca.pem")(opt)
	assert.Equal(t, "/path/to/ca.pem", opt.caFilePath)
}

func TestWithCAContent(t *testing.T) {
	opt := &newOptionalArg{}
	testContent := "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"
	WithCAContent(testContent)(opt)
	assert.Equal(t, testContent, opt.caContent)
}
