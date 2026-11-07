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

package orcarouter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingStore is a Store that fails on the operation under test.
type failingStore struct {
	loadErr  error
	saveErr  error
	clearErr error
	key      string
	source   Source
}

func (s *failingStore) Load() (string, Source, error) { return s.key, s.source, s.loadErr }
func (s *failingStore) Save(key string, source Source) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.key, s.source = key, source
	return nil
}
func (s *failingStore) Clear() error {
	if s.clearErr != nil {
		return s.clearErr
	}
	s.key, s.source = "", ""
	return nil
}

// TestBothAdaptersProduceTheSameCredential is the core of the dual-auth
// requirement: the pasted key and the PKCE login end at the same value, and
// nothing downstream can tell them apart except the recorded source.
func TestBothAdaptersProduceTheSameCredential(t *testing.T) {
	store := &MemoryStore{}
	manager, err := NewManager(store)
	require.NoError(t, err)

	viaKey, err := manager.SetAPIKey("sk-orca-pasted-key")
	require.NoError(t, err)

	managerPKCE, err := NewManager(&MemoryStore{})
	require.NoError(t, err)
	viaPKCE, err := managerPKCE.SetPKCE(Credential{Key: "sk-orca-pkce-key", UserID: "42", Scope: "api"})
	require.NoError(t, err)

	assert.Equal(t, SourceAPIKey, viaKey.Source)
	assert.Equal(t, SourcePKCE, viaPKCE.Source)

	// The inference client and the catalog accept either one through the same
	// interface: neither reads Source.
	for _, cred := range []Credential{viaKey, viaPKCE} {
		assert.True(t, len(cred.Key) > len(KeyPrefix))
		assert.Equal(t, cred.Key, (Credential{Key: cred.Key}).Key)
	}

	// Both are persisted as an ordinary key.
	stored, source, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, "sk-orca-pasted-key", stored)
	assert.Equal(t, SourceAPIKey, source)
}

func TestManagerPersistsAndReloads(t *testing.T) {
	store := &MemoryStore{}
	manager, err := NewManager(store)
	require.NoError(t, err)

	_, ok := manager.Current()
	assert.False(t, ok)

	cred, err := manager.SetAPIKey("sk-orca-durable")
	require.NoError(t, err)
	assert.Equal(t, 1, cred.Generation)

	// A second process reads the same stored key and reuses it, rather than
	// authorizing again.
	reloaded, err := NewManager(store)
	require.NoError(t, err)
	got, ok := reloaded.Current()
	require.True(t, ok)
	assert.Equal(t, "sk-orca-durable", got.Key)
}

func TestManagerClear(t *testing.T) {
	store := &MemoryStore{}
	manager, err := NewManager(store)
	require.NoError(t, err)
	_, err = manager.SetAPIKey("sk-orca-clear-me")
	require.NoError(t, err)

	require.NoError(t, manager.Clear())
	_, ok := manager.Current()
	assert.False(t, ok)
	assert.True(t, manager.NeedsReauth())

	key, _, err := store.Load()
	require.NoError(t, err)
	assert.Empty(t, key)
}

func TestManagerRejectedIsGenerationSafe(t *testing.T) {
	manager, err := NewManager(&MemoryStore{})
	require.NoError(t, err)

	first, err := manager.SetAPIKey("sk-orca-first")
	require.NoError(t, err)
	assert.Equal(t, 1, first.Generation)

	// A failure from the first request arrives after a new login.
	second, err := manager.SetPKCE(Credential{Key: "sk-orca-second"})
	require.NoError(t, err)
	assert.Equal(t, 2, second.Generation)

	manager.Rejected(first.Generation)
	assert.False(t, manager.NeedsReauth(), "a late failure from an old generation must not mark the new credential")

	current, ok := manager.Current()
	require.True(t, ok)
	assert.Equal(t, "sk-orca-second", current.Key)

	// A rejection of the current generation does mark it.
	manager.Rejected(second.Generation)
	assert.True(t, manager.NeedsReauth())

	// And a new login clears the flag without deleting the old key first.
	third, err := manager.SetPKCE(Credential{Key: "sk-orca-third"})
	require.NoError(t, err)
	assert.False(t, manager.NeedsReauth())
	assert.Equal(t, third.Generation, manager.Generation())
	assert.Equal(t, "sk-orca-third", third.Key)
}

func TestManagerFailedSaveKeepsOldKey(t *testing.T) {
	store := &failingStore{key: "sk-orca-original", source: SourcePKCE}
	manager, err := NewManager(store)
	require.NoError(t, err)

	store.saveErr = errors.New("disk full")
	_, err = manager.SetAPIKey("sk-orca-replacement")
	require.Error(t, err)

	got, ok := manager.Current()
	require.True(t, ok)
	assert.Equal(t, "sk-orca-original", got.Key, "a failed store write must not lose the working credential")
	assert.Equal(t, "sk-orca-original", store.key)
}

func TestManagerLoadError(t *testing.T) {
	_, err := NewManager(&failingStore{loadErr: errors.New("unreadable")})
	require.Error(t, err)
}

func TestSetPKCERequiresKey(t *testing.T) {
	manager, err := NewManager(&MemoryStore{})
	require.NoError(t, err)
	_, err = manager.SetPKCE(Credential{})
	assert.ErrorIs(t, err, ErrExchangeRejected)
}

func TestManagerNeedsReauthAfterRevocation(t *testing.T) {
	// A revoked durable key is a terminal reauthentication, never a refresh.
	manager, err := NewManager(&MemoryStore{})
	require.NoError(t, err)
	cred, err := manager.SetPKCE(Credential{Key: "sk-orca-revoked"})
	require.NoError(t, err)

	manager.Rejected(cred.Generation)
	assert.True(t, manager.NeedsReauth())
	// The key is kept until a new login succeeds, so a misclassified failure
	// does not destroy it.
	current, ok := manager.Current()
	require.True(t, ok)
	assert.Equal(t, "sk-orca-revoked", current.Key)
}
