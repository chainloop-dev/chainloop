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

package integration

import (
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/integrations/gen/dependencytrack/cyclonedx/v1"
	"github.com/stretchr/testify/assert"
)

func TestValidateConfiguration(t *testing.T) {
	testIntegrationConfig := func(allowAutoCreate bool) *v1.RegistrationConfig {
		return &v1.RegistrationConfig{
			AllowAutoCreate: allowAutoCreate,
			Domain:          "domain",
		}
	}

	testAttachmentConfig := func(projectID, projectName string) *v1.AttachmentConfig {
		if projectID != "" {
			return &v1.AttachmentConfig{
				Project: &v1.AttachmentConfig_ProjectId{ProjectId: projectID},
			}
		} else if projectName != "" {
			return &v1.AttachmentConfig{
				Project: &v1.AttachmentConfig_ProjectName{ProjectName: projectName},
			}
		}

		return &v1.AttachmentConfig{}
	}

	tests := []struct {
		integrationConfig *v1.RegistrationConfig
		attachmentConfig  *v1.AttachmentConfig
		errorMsg          string
	}{
		{nil, nil, "invalid configuration"},
		// autocreate required but not supported
		{testIntegrationConfig(false), testAttachmentConfig("", "new-project"), "auto creation of projects is not supported in this integration"},
		// autocreate required and supported
		{testIntegrationConfig(true), testAttachmentConfig("", "new-project"), ""},
		// Neither projectID nor autocreate provided
		{testIntegrationConfig(false), testAttachmentConfig("", ""), "AttachmentConfig.Project: value is required"},
		// project ID provided
		{testIntegrationConfig(false), testAttachmentConfig("pid", ""), ""},
	}

	for _, tc := range tests {
		t.Run(tc.errorMsg, func(t *testing.T) {
			err := validateAttachmentConfiguration(tc.integrationConfig, tc.attachmentConfig)
			if tc.errorMsg != "" {
				assert.ErrorContains(t, err, tc.errorMsg)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
