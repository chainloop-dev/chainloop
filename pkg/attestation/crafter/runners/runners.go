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

package runners

import (
	"fmt"
	"os"
)

type EnvVarDefinition struct {
	Name     string
	Optional bool
}

func resolveEnvVars(envVarsDefinitions []*EnvVarDefinition) (map[string]string, []*error) {
	result := make(map[string]string)
	var errors []*error

	for _, envVarDef := range envVarsDefinitions {
		value := os.Getenv(envVarDef.Name)
		if value != "" {
			result[envVarDef.Name] = value
			continue
		}

		if !envVarDef.Optional {
			err := fmt.Errorf("environment variable %s cannot be resolved", envVarDef.Name)
			errors = append(errors, &err)
		}
	}

	if len(errors) > 0 {
		result = nil
	}

	return result, errors
}
