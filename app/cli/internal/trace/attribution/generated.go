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

package attribution

import (
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// FilterGenerated removes files matched by isGenerated from changes.Files and
// subtracts their contributions from the aggregate line and file-count totals,
// so generated code does not skew downstream AI/human attribution. A nil
// isGenerated leaves changes untouched.
func FilterGenerated(changes *aicodingsession.CodeChanges, isGenerated func(path string) bool) {
	if isGenerated == nil {
		return
	}
	filtered := changes.Files[:0]
	for _, f := range changes.Files {
		if isGenerated(f.Path) {
			changes.LinesAdded -= f.LinesAdded
			changes.LinesRemoved -= f.LinesRemoved
			switch f.Status {
			case "created":
				changes.FilesCreated--
			case "deleted":
				changes.FilesDeleted--
			default:
				changes.FilesModified--
			}

			continue
		}
		filtered = append(filtered, f)
	}
	changes.Files = filtered
}
