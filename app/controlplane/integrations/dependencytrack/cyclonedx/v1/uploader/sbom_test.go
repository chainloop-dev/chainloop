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

package uploader

import (
	"bytes"
	"io"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const hostname = "http://example.com"

func TestNewSBOMUploader(t *testing.T) {
	const projectID = "existing-project-id"
	const projectName = "new project name"
	var sbomReader = bytes.NewBuffer(nil)

	tests := []struct {
		hostname, apiKey, projectID, projectName string
		sbom                                     io.Reader
		wantError                                bool
	}{
		// no api key
		{hostname, "", projectID, projectName, sbomReader, true},
		// invalid hostname
		{"invalid-hostname", "apikey", projectID, projectName, sbomReader, true},
		// both projectID and name
		{hostname, "apikey", projectID, projectName, sbomReader, true},
		{hostname, "apikey", projectID, "", sbomReader, false},
		{hostname, "apikey", "", projectName, sbomReader, false},
	}

	assert := assert.New(t)
	for _, tc := range tests {
		got, err := NewSBOMUploader(tc.hostname, tc.apiKey, tc.sbom, tc.projectID, tc.projectName)
		if tc.wantError {
			assert.Error(err)
			continue
		}

		uri, err := url.Parse(tc.hostname)
		assert.NoError(err)
		assert.EqualValues(&SBOMUploader{
			&base{
				apiKey: tc.apiKey,
				host:   uri,
			},
			tc.sbom,
			tc.projectID, tc.projectName,
		}, got)
	}
}

func TestNewChecker(t *testing.T) {
	tests := []struct {
		hostname   string
		apiKey     string
		autoCreate bool
		wantError  bool
	}{
		// no api key
		{hostname, "", true, true},
		// invalid hostname
		{"invalid-hostname", "apikey", true, true},
		// valid arguments
		{hostname, "apikey", true, false},
		{hostname, "apikey", false, false},
	}

	assert := assert.New(t)
	for _, tc := range tests {
		got, err := NewIntegration(tc.hostname, tc.apiKey, tc.autoCreate)
		if tc.wantError {
			assert.Error(err)
			continue
		}

		uri, err := url.Parse(tc.hostname)
		assert.NoError(err)
		assert.EqualValues(&Integration{
			base: &base{
				apiKey: tc.apiKey,
				host:   uri,
			},
			checkAutoCreate: tc.autoCreate,
		}, got)
	}
}

func TestDoCheck(t *testing.T) {
	tests := []struct {
		teamPermissions []string
		WithAutoCreate  bool
		wantValid       bool
	}{
		{[]string{}, false, false},
		{[]string{bomUploadPermission}, false, false},
		{[]string{bomUploadPermission, viewPortfolioPermission}, false, true},
		{[]string{projectCreationPermission}, false, false},
		{[]string{bomUploadPermission}, true, false},
		{[]string{bomUploadPermission, viewPortfolioPermission, projectCreationPermission}, true, true},
	}

	for _, tc := range tests {
		err := doCheck(tc.teamPermissions, tc.WithAutoCreate)
		if !tc.wantValid {
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrValidation)
		} else {
			assert.Nil(t, err)
		}
	}
}
