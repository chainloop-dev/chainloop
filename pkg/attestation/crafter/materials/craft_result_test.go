//
// Copyright 2026 The Chainloop Authors.
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

package materials

import (
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
)

// craftedMaterial unwraps a Craft call for the tests that assert only on the
// material. It keeps the (value, error) shape so wrapping a call changes nothing
// about how the error is handled:
//
//	got, err := craftedMaterial(crafter.Craft(ctx, path))
//
// Tests that care about what the crafter stored in place of the artifact use
// CraftResult.Transformed directly instead.
func craftedMaterial(res *CraftResult, err error) (*api.Attestation_Material, error) {
	if res == nil {
		return nil, err
	}
	return res.Material, err
}
