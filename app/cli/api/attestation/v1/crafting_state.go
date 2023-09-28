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

package v1

import (
	"errors"
	"fmt"
)

type NormalizedMaterialOutput struct {
	Name, Digest string
	IsOutput     bool
	Content      []byte
}

// NormalizedOutput returns a common representation of the properties of a material
// regardless of how it's been encoded.
// For example, it's common to have materials based on artifacts, so we want to normalize the output
func (m *Attestation_Material) NormalizedOutput() (*NormalizedMaterialOutput, error) {
	if m == nil {
		return nil, errors.New("material not provided")
	}

	if a := m.GetContainerImage(); a != nil {
		return &NormalizedMaterialOutput{a.Name, a.Digest, a.IsSubject, nil}, nil
	}

	if a := m.GetString_(); a != nil {
		return &NormalizedMaterialOutput{Content: []byte(a.Value)}, nil
	}

	if a := m.GetArtifact(); a != nil {
		return &NormalizedMaterialOutput{a.Name, a.Digest, a.IsSubject, a.Content}, nil
	}

	return nil, fmt.Errorf("unknown material: %s", m.MaterialType)
}
