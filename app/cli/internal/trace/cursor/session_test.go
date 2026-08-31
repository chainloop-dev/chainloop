package cursor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeRepoPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/Users/me/projects/myrepo", "Users-me-projects-myrepo"},
		{"/home/user/project-name", "home-user-project-name"},
		{"/a//b", "a-b"},
		{"/a/b.c/d", "a-b-c-d"},
		{"", ""},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, sanitizeRepoPath(tc.in), "sanitizeRepoPath(%q)", tc.in)
	}
}

func TestResolveSessionJSONLFlat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abc123.jsonl")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0600))

	got, err := resolveSessionJSONL(dir, "abc123")
	require.NoError(t, err)
	require.Equal(t, path, got)
}

func TestResolveSessionJSONLNested(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "abc123")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	path := filepath.Join(nestedDir, "abc123.jsonl")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0600))

	got, err := resolveSessionJSONL(dir, "abc123")
	require.NoError(t, err)
	require.Equal(t, path, got)
}

func TestResolveSessionJSONLNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveSessionJSONL(dir, "missing")
	require.Error(t, err, "expected error for missing transcript")
	require.ErrorIs(t, err, os.ErrNotExist)
}
