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

package action

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculatePropertiesMap(t *testing.T) {
	testCases := []struct {
		schemaPath string
		want       SchemaPropertiesMap
	}{
		{
			"basic.json",
			SchemaPropertiesMap{
				"allowAutoCreate": {
					Name:        "allowAutoCreate",
					Description: "Support of creating projects on demand",
					Type:        "boolean",
					Required:    false,
				},
				"apiKey": {
					Name:        "apiKey",
					Description: "The API key to use for authentication",
					Type:        "string",
					Required:    true,
				},
				"instanceURI": {
					Name:        "instanceURI",
					Description: "The URL of the Dependency-Track instance",
					Type:        "string",
					Required:    true,
					Format:      "uri",
				},
				"port": {
					Name: "port",
					Type: "number",
				},
			},
		},
		{
			// NOTE: oneof work in the validation but are not shown in the map
			// This testCase is here to document this limitation
			"oneof_required.json",
			SchemaPropertiesMap{
				"projectID": {
					Name:        "projectID",
					Description: "The ID of the existing project to send the SBOMs to",
					Type:        "string",
				},
				"projectName": {
					Name:        "projectName",
					Description: "The name of the project to create and send the SBOMs to",
					Type:        "string",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.schemaPath, func(t *testing.T) {
			schemaRaw, err := os.ReadFile(fmt.Sprintf("testdata/schemas/%s", tc.schemaPath))
			require.NoError(t, err)
			schema, err := compileJSONSchema(schemaRaw)
			require.NoError(t, err)

			var got = make(SchemaPropertiesMap)
			err = calculatePropertiesMap(schema, &got)
			assert.NoError(t, err)

			assert.Equal(t, tc.want, got)
		})
	}
}
