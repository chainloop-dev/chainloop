package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellPreSignatureRoundTrip(t *testing.T) {
	store := NewGitStore(t.TempDir())
	sessionID := "sess-123"
	sig := map[string]string{
		"a.go":       "hash-a",
		"sub/b.json": "hash-b",
	}

	require.NoError(t, store.SaveShellPreSignature(sessionID, sig))

	loaded, err := store.LoadShellPreSignature(sessionID)
	require.NoError(t, err)
	assert.Equal(t, sig, loaded)

	store.DeleteShellPreSignature(sessionID)

	_, err = store.LoadShellPreSignature(sessionID)
	assert.Error(t, err, "signature should be gone after delete")
}
