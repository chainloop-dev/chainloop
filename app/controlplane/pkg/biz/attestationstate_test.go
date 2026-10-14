//
// Copyright 2024-2026 The Chainloop Authors.
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

package biz

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecryptWrongPassphraseReturnsTypedError(t *testing.T) {
	t.Parallel()

	ciphertext, err := encrypt([]byte("hello attestation"), "correct-passphrase")
	require.NoError(t, err)

	// The passphrase that created the ciphertext still decrypts it.
	plaintext, err := decrypt(ciphertext, "correct-passphrase")
	require.NoError(t, err)
	assert.Equal(t, []byte("hello attestation"), plaintext)

	// A different passphrase is reported as a typed error so the service layer
	// can turn it into a client error instead of masking it as a server error.
	_, err = decrypt(ciphertext, "another-passphrase")
	require.Error(t, err)
	assert.True(t, IsErrInvalidPassphrase(err), "expected ErrInvalidPassphrase, got %T: %v", err, err)
}

// The read path wraps the decryption error with %w, so the typed error must
// still be discoverable through errors.As.
func TestErrInvalidPassphraseIsDiscoverableWhenWrapped(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("failed to decrypt attestation state: %w", NewErrInvalidPassphrase())
	assert.True(t, IsErrInvalidPassphrase(wrapped))
}
