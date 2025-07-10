//
// Copyright 2025 The Chainloop Authors.
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
	"fmt"

	"github.com/google/uuid"
)

// Parse is a helper method to parse an IdentityReference from the protobuf message.
func (i *IdentityReference) Parse() (*uuid.UUID, *string, error) {
	if i.GetId() != "" && i.GetName() != "" {
		return nil, nil, fmt.Errorf("cannot provide both ID and name")
	}

	if i.GetId() != "" {
		identityUUID, err := uuid.Parse(i.GetId())
		if err != nil {
			return nil, nil, fmt.Errorf("invalid identity ID")
		}
		return &identityUUID, nil, nil
	} else if i.GetName() != "" {
		identityName := i.GetName()
		return nil, &identityName, nil
	}

	return nil, nil, nil
}

func (i *IdentityReference) IsSet() bool {
	if i == nil {
		return false
	}

	return i.GetId() != "" || i.GetName() != ""
}
