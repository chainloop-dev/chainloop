package attribution

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterGenerated(t *testing.T) {
	t.Run("nil matcher leaves changes untouched", func(t *testing.T) {
		changes := &aicodingsession.CodeChanges{
			FilesModified: 1,
			LinesAdded:    5,
			Files:         []aicodingsession.FileChange{{Path: "a.go", Status: "modified", LinesAdded: 5}},
		}

		FilterGenerated(changes, nil)

		assert.Len(t, changes.Files, 1)
		assert.Equal(t, 5, changes.LinesAdded)
		assert.Equal(t, 1, changes.FilesModified)
	})

	t.Run("generated created file is dropped and counters decremented", func(t *testing.T) {
		isGenerated := func(path string) bool { return path == "gen/out.pb.go" }
		changes := &aicodingsession.CodeChanges{
			FilesModified: 1,
			FilesCreated:  1,
			LinesAdded:    203,
			LinesRemoved:  5,
			Files: []aicodingsession.FileChange{
				{Path: "a.go", Status: "modified", LinesAdded: 3, LinesRemoved: 5},
				{Path: "gen/out.pb.go", Status: "created", LinesAdded: 200, LinesRemoved: 0},
			},
		}

		FilterGenerated(changes, isGenerated)

		assert.Len(t, changes.Files, 1)
		assert.Equal(t, "a.go", changes.Files[0].Path)
		assert.Equal(t, 3, changes.LinesAdded)
		assert.Equal(t, 5, changes.LinesRemoved)
		assert.Equal(t, 1, changes.FilesModified)
		assert.Equal(t, 0, changes.FilesCreated)
	})

	t.Run("generated deleted file decrements FilesDeleted", func(t *testing.T) {
		isGenerated := func(path string) bool { return path == "old.pb.go" }
		changes := &aicodingsession.CodeChanges{
			FilesDeleted: 1,
			LinesRemoved: 7,
			Files: []aicodingsession.FileChange{
				{Path: "old.pb.go", Status: "deleted", LinesRemoved: 7},
			},
		}

		FilterGenerated(changes, isGenerated)

		assert.Empty(t, changes.Files)
		assert.Equal(t, 0, changes.FilesDeleted)
		assert.Equal(t, 0, changes.LinesRemoved)
	})

	t.Run("ent gitattributes rules drop generated ent code but keep schema and migrations", func(t *testing.T) {
		dir := t.TempDir()
		// Order matches the project .gitattributes: general rule first, then
		// more-specific overrides (last match wins, per gitattributes spec).
		writeGitAttributes(t, dir,
			"backend/internal/data/ent/** linguist-generated=true\n"+
				"backend/internal/data/ent/migrate/migrations/** linguist-generated=false\n"+
				"backend/internal/data/ent/schema/* linguist-generated=false\n",
		)

		isGenerated := tracegit.NewGoGitClient().GeneratedMatcher(dir)

		changes := &aicodingsession.CodeChanges{
			FilesCreated:  3,
			FilesModified: 4,
			FilesDeleted:  1,
			LinesAdded:    100 + 50 + 200 + 10 + 7 + 25 + 40,
			LinesRemoved:  5 + 2 + 30 + 1 + 0 + 3 + 8 + 4,
			Files: []aicodingsession.FileChange{
				// Generated ent code — should be dropped.
				{Path: "backend/internal/data/ent/client.go", Status: "modified", LinesAdded: 100, LinesRemoved: 5},
				{Path: "backend/internal/data/ent/account.go", Status: "modified", LinesAdded: 50, LinesRemoved: 2},
				// Generated files directly under migrate/ — must also be dropped (the bug we're locking in).
				{Path: "backend/internal/data/ent/migrate/migrate.go", Status: "modified", LinesAdded: 200, LinesRemoved: 30},
				{Path: "backend/internal/data/ent/migrate/schema.go", Status: "deleted", LinesAdded: 0, LinesRemoved: 8},
				// Hand-managed migrations — must be kept.
				{Path: "backend/internal/data/ent/migrate/migrations/20240602211927.sql", Status: "created", LinesAdded: 10, LinesRemoved: 1},
				{Path: "backend/internal/data/ent/migrate/migrations/atlas.sum", Status: "modified", LinesAdded: 7, LinesRemoved: 0},
				// Schema source — must be kept.
				{Path: "backend/internal/data/ent/schema/ai_coding_session.go", Status: "created", LinesAdded: 25, LinesRemoved: 3},
				// Unrelated path — must be kept.
				{Path: "backend/internal/service/evidence.go", Status: "created", LinesAdded: 40, LinesRemoved: 4},
			},
		}

		FilterGenerated(changes, isGenerated)

		gotPaths := make([]string, 0, len(changes.Files))
		for _, f := range changes.Files {
			gotPaths = append(gotPaths, f.Path)
		}
		assert.ElementsMatch(t, []string{
			"backend/internal/data/ent/migrate/migrations/20240602211927.sql",
			"backend/internal/data/ent/migrate/migrations/atlas.sum",
			"backend/internal/data/ent/schema/ai_coding_session.go",
			"backend/internal/service/evidence.go",
		}, gotPaths)

		// Surviving line totals: 10+7+25+40 added, 1+0+3+4 removed.
		assert.Equal(t, 82, changes.LinesAdded)
		assert.Equal(t, 8, changes.LinesRemoved)
		// Surviving status counts: 3 created (sql, schema, evidence), 1 modified (atlas.sum), 0 deleted.
		assert.Equal(t, 3, changes.FilesCreated)
		assert.Equal(t, 1, changes.FilesModified)
		assert.Equal(t, 0, changes.FilesDeleted)
	})
}

func writeGitAttributes(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ".gitattributes")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600), "write .gitattributes")
}
