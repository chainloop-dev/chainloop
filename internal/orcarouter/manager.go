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
	"fmt"
	"sync"
)

// ErrNoCredential is returned when no OrcaRouter credential is configured.
var ErrNoCredential = errors.New("no OrcaRouter credential is configured")

// Store is the persistence seam. The CLI backs it with the configuration file
// it already trusts; tests back it with an in-memory implementation. Only the
// key string and its source are persisted: there is no refresh token to keep,
// because a PKCE login yields a durable API key.
type Store interface {
	// Load returns the stored key and the adapter that produced it. A missing
	// entry returns an empty key and no error.
	Load() (key string, source Source, err error)
	// Save writes the key and its source.
	Save(key string, source Source) error
	// Clear removes the entry.
	Clear() error
}

// MemoryStore is a Store that keeps the credential in memory, for tests and for
// callers that do not persist anything.
type MemoryStore struct {
	mu     sync.Mutex
	key    string
	source Source
}

// Load implements Store.
func (s *MemoryStore) Load() (string, Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.key, s.source, nil
}

// Save implements Store.
func (s *MemoryStore) Save(key string, source Source) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key, s.source = key, source
	return nil
}

// Clear implements Store.
func (s *MemoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key, s.source = "", ""
	return nil
}

// Manager owns the current credential and its generation. The two adapters
// produce the same Credential, so everything downstream is adapter-agnostic.
//
// A revoked key is durable and cannot be refreshed. When a request is rejected,
// the manager marks exactly the generation that made it as needing
// reauthentication, and leaves a newer credential alone: a late failure from an
// old request must not break a fresh login.
type Manager struct {
	mu         sync.Mutex
	store      Store
	credential Credential
	needsReaut bool
}

// NewManager reads the stored credential. A storage failure is reported but
// does not prevent the caller from logging in again.
func NewManager(store Store) (*Manager, error) {
	m := &Manager{store: store}
	if store == nil {
		return m, nil
	}
	key, source, err := store.Load()
	if err != nil {
		return m, fmt.Errorf("read the OrcaRouter credential: %w", err)
	}
	if key != "" {
		m.credential = Credential{Key: key, Source: source, Generation: 1}
	}
	return m, nil
}

// Current returns the credential in use, if any.
func (m *Manager) Current() (Credential, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.credential.Key == "" {
		return Credential{}, false
	}
	return m.credential, true
}

// NeedsReauth reports whether the current credential was rejected.
func (m *Manager) NeedsReauth() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.needsReaut
}

// SetAPIKey validates and stores a pasted key, through the same path a PKCE
// login ends at.
func (m *Manager) SetAPIKey(key string) (Credential, error) {
	cred, err := (APIKeyAdapter{}).Acquire(key)
	if err != nil {
		return Credential{}, err
	}
	return m.adopt(cred)
}

// SetPKCE adopts a credential minted by a PKCE login.
func (m *Manager) SetPKCE(cred Credential) (Credential, error) {
	if cred.Key == "" {
		return Credential{}, ErrExchangeRejected
	}
	cred.Source = SourcePKCE
	return m.adopt(cred)
}

// adopt installs a new credential, advancing the generation and clearing the
// reauthentication flag. The stored key is replaced only after the save
// succeeds, so a failed write does not lose the old one.
func (m *Manager) adopt(cred Credential) (Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store != nil {
		if err := m.store.Save(cred.Key, cred.Source); err != nil {
			return Credential{}, fmt.Errorf("store the OrcaRouter credential: %w", err)
		}
	}
	cred.Generation = m.credential.Generation + 1
	m.credential = cred
	m.needsReaut = false
	return cred, nil
}

// Clear removes the credential from memory and from storage.
func (m *Manager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.store != nil {
		if err := m.store.Clear(); err != nil {
			return fmt.Errorf("clear the OrcaRouter credential: %w", err)
		}
	}
	m.credential = Credential{}
	m.needsReaut = true
	return nil
}

// Rejected marks the credential of the given generation as needing
// reauthentication. A rejection for a generation that is no longer current, or
// for a key that is no longer the stored one, changes nothing, so a late
// failure from an old request cannot poison a fresh login.
func (m *Manager) Rejected(generation int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.credential.Key == "" {
		return
	}
	if generation != 0 && generation != m.credential.Generation {
		return
	}
	m.needsReaut = true
}

// Generation returns the current credential generation, or zero when there is
// none. A caller records it before a request and passes it back to Rejected.
func (m *Manager) Generation() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.credential.Generation
}
