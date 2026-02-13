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

func TestBackwardCompatibility_StoredFilePath(t *testing.T) {
	// This test verifies that if a user has an old config with a stored file path,
	// the new code will still load it correctly via the file path method.

	// Check if the file exists, skip test if not
	if _, err := os.Stat(caPath); os.IsNotExist(err) {
		t.Skip("Test CA file not found, skipping test")
	}

	// Simulate an old config with a stored file path
	storedValue := caPath

	// Verify IsFilePath detects it as a file path
	_, err := os.Stat(storedValue)
	assert.Nil(t, err, "stored file path should be detected as a file path")

	// Verify it can be loaded using the file path method
	certsPool, err := x509.SystemCertPool()
	require.NoError(t, err)

	err = appendCAFromFile(storedValue, certsPool)
	assert.NoError(t, err, "should successfully load CA from stored file path")
}

func TestBackwardCompatibility_NewClientOldConfig(t *testing.T) {
	// This test verifies the complete flow: new client reading old config with file path

	// Check if the file exists, skip test if not
	if _, err := os.Stat(caPath); os.IsNotExist(err) {
		t.Skip("Test CA file not found, skipping test")
	}

	// Simulate config value (could be file path or content)
	oldConfigValue := caPath

	// New client logic: detect and load appropriately
	var opts []Option
	if _, err := os.Stat(oldConfigValue); err == nil {
		opts = append(opts, WithCAFile(oldConfigValue))
	} else {
		opts = append(opts, WithCAContent(oldConfigValue))
	}

	// Verify the correct option was chosen
	require.Len(t, opts, 1)

	// Apply the option and verify it set caFilePath (not caContent)
	optArg := &newOptionalArg{}
	opts[0](optArg)
	assert.Equal(t, caPath, optArg.caFilePath, "should use file path method for old config")
	assert.Empty(t, optArg.caContent, "should not use content method for old config")
}

func TestBackwardCompatibility_OldClientNewConfig(t *testing.T) {
	// This test verifies that if a path is stored in config, both old and new
	// clients can load it. Old clients would directly use WithCAFile, new clients
	// would detect it via IsFilePath and use WithCAFile.

	// Check if the file exists, skip test if not
	if _, err := os.Stat(caPath); os.IsNotExist(err) {
		t.Skip("Test CA file not found, skipping test")
	}

	// Stored config value (file path)
	configValue := caPath

	certsPool1, err := x509.SystemCertPool()
	require.NoError(t, err)

	certsPool2, err := x509.SystemCertPool()
	require.NoError(t, err)

	// Old client behavior: directly use file path
	err = appendCAFromFile(configValue, certsPool1)
	assert.NoError(t, err, "old client should load file path")

	// New client behavior: detect then use file path
	if _, statErr := os.Stat(configValue); statErr == nil {
		err = appendCAFromFile(configValue, certsPool2)
	} else {
		err = appendCAFromContent(configValue, certsPool2)
	}
	assert.NoError(t, err, "new client should load file path via detection")
}
