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

package biz

import (
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/stretchr/testify/assert"
)

func TestValidateDependendyTrackAttachment(t *testing.T) {
	testIntegrationConfig := func(allowAutoCreate bool) *v1.IntegrationConfig {
		return &v1.IntegrationConfig{
			Config: &v1.IntegrationConfig_DependencyTrack_{
				DependencyTrack: &v1.IntegrationConfig_DependencyTrack{
					AllowAutoCreate: allowAutoCreate,
				},
			},
		}
	}

	testAttachmentConfig := func(projectID, projectName string) *v1.IntegrationAttachmentConfig {
		var projectConfig *v1.IntegrationAttachmentConfig_DependencyTrack
		if projectID != "" {
			projectConfig = &v1.IntegrationAttachmentConfig_DependencyTrack{
				Project: &v1.IntegrationAttachmentConfig_DependencyTrack_ProjectId{ProjectId: projectID},
			}
		} else if projectName != "" {
			projectConfig = &v1.IntegrationAttachmentConfig_DependencyTrack{
				Project: &v1.IntegrationAttachmentConfig_DependencyTrack_ProjectName{ProjectName: projectName},
			}
		}

		return &v1.IntegrationAttachmentConfig{
			Config: &v1.IntegrationAttachmentConfig_DependencyTrack_{
				DependencyTrack: projectConfig,
			},
		}
	}

	tests := []struct {
		integrationConfig *v1.IntegrationConfig
		attachmentConfig  *v1.IntegrationAttachmentConfig
		errorMsg          string
	}{
		{nil, nil, "invalid configuration"},
		// autocreate required but not supported
		{testIntegrationConfig(false), testAttachmentConfig("", "new-project"), "auto creation of projects is not supported in this integration"},
		// autocreate required and supported
		{testIntegrationConfig(true), testAttachmentConfig("", "new-project"), ""},
		// Neither projectID nor autocreate provided
		{testIntegrationConfig(false), testAttachmentConfig("", ""), "invalid configurations"},
		// project ID provided
		{testIntegrationConfig(false), testAttachmentConfig("pid", ""), ""},
	}

	for _, tc := range tests {
		err := tc.integrationConfig.GetDependencyTrack().ValidateAttachment(tc.attachmentConfig.GetDependencyTrack())
		if tc.errorMsg != "" {
			assert.ErrorContains(t, err, tc.errorMsg)
		} else {
			assert.Nil(t, err)
		}
	}
}
