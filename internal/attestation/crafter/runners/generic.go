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
	"os"
)

type Generic struct{}

const GenericID = "generic"

func NewGeneric() *Generic {
	return &Generic{}
}

func (r *Generic) CheckEnv() bool {
	return true
}

// Returns a list of environment variables names. These lists are used to
// automatically inject environment variables into the attestation.

func (r *Generic) ListEnvVars() []string {
	return []string{}
}

func (r *Generic) ListOptionalEnvVars() []string {
	return []string{}
}

func (r *Generic) ResolveEnvVars() map[string]string {
	return make(map[string]string)
}

func (r *Generic) String() string {
	return GenericID
}

func (r *Generic) RunURI() string {
	return ""
}

func resolveEnvVars(requiredEnvVars, optionalEnvVars []string) map[string]string {
	result := make(map[string]string)

	for _, name := range requiredEnvVars {
		value := os.Getenv(name)
		result[name] = value
	}

	for _, name := range optionalEnvVars {
		value := os.Getenv(name)
		result[name] = value
	}

	return result
}
