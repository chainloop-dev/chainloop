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

package conf

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

func (c *ReferrerSharedIndex) ValidateOrgs() error {
	if c == nil || !c.Enabled {
		return nil
	}

	if c.Enabled && len(c.AllowedOrgs) == 0 {
		return errors.New("index is enabled, but no orgs are allowed")
	}

	for _, orgID := range c.AllowedOrgs {
		if _, err := uuid.Parse(orgID); err != nil {
			return fmt.Errorf("invalid org id: %s", orgID)
		}
	}

	return nil
}
