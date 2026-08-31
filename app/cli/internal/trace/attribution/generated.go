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
