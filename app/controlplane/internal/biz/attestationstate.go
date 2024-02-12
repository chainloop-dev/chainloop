//
// Copyright 2024 The Chainloop Authors.
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
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/google/uuid"
	"golang.org/x/crypto/pbkdf2"
	"google.golang.org/protobuf/proto"
)

type AttestationState struct {
	State *v1.CraftingState
}

type AttestationStateRepo interface {
	Initialized(ctx context.Context, workflowRunID uuid.UUID) (bool, error)
	Save(ctx context.Context, workflowRunID uuid.UUID, state []byte) error
	Read(ctx context.Context, workflowRunID uuid.UUID) ([]byte, error)
	Reset(ctx context.Context, workflowRunID uuid.UUID) error
}

type AttestationStateUseCase struct {
	repo      AttestationStateRepo
	wfRunRepo WorkflowRunRepo
}

func NewAttestationStateUseCase(repo AttestationStateRepo, wfRunRepo WorkflowRunRepo) (*AttestationStateUseCase, error) {
	return &AttestationStateUseCase{repo, wfRunRepo}, nil
}

func (uc *AttestationStateUseCase) Initialized(ctx context.Context, workflowID, runID string) (bool, error) {
	runUUID, err := uc.checkWorkflowRunInWorkflow(ctx, workflowID, runID)
	if err != nil {
		return false, fmt.Errorf("failed to check workflow run: %w", err)
	}

	initialized, err := uc.repo.Initialized(ctx, *runUUID)
	if err != nil {
		return false, fmt.Errorf("failed to check initialized state: %w", err)
	}

	return initialized, nil
}

func (uc *AttestationStateUseCase) Save(ctx context.Context, workflowID, runID string, state *v1.CraftingState, passphrase string) error {
	runUUID, err := uc.checkWorkflowRunInWorkflow(ctx, workflowID, runID)
	if err != nil {
		return fmt.Errorf("failed to check workflow run: %w", err)
	}

	rawState, err := proto.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal attestation state: %w", err)
	}

	encryptedState, err := encrypt(rawState, passphrase)
	if err != nil {
		return fmt.Errorf("failed to encrypt attestation state: %w", err)
	}

	if err := uc.repo.Save(ctx, *runUUID, encryptedState); err != nil {
		return fmt.Errorf("failed to save attestation state: %w", err)
	}

	return nil
}

func (uc *AttestationStateUseCase) Read(ctx context.Context, workflowID, runID, passphrase string) (*AttestationState, error) {
	runUUID, err := uc.checkWorkflowRunInWorkflow(ctx, workflowID, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to check workflow run: %w", err)
	}

	res, err := uc.repo.Read(ctx, *runUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to read attestation state: %w", err)
	}

	decryptedState, err := decrypt(res, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt attestation state: %w", err)
	}

	state := &v1.CraftingState{}
	if err := proto.Unmarshal(decryptedState, state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attestation state: %w", err)
	}

	return &AttestationState{State: state}, nil
}

func (uc *AttestationStateUseCase) Reset(ctx context.Context, workflowID, runID string) error {
	runUUID, err := uc.checkWorkflowRunInWorkflow(ctx, workflowID, runID)
	if err != nil {
		return fmt.Errorf("failed to check workflow run: %w", err)
	}

	if err := uc.repo.Reset(ctx, *runUUID); err != nil {
		return fmt.Errorf("failed to reset attestation state: %w", err)
	}

	return nil
}

// checkWorkflowRunInWorkflow checks if the workflow run belongs to the provided workflow
// This is important because the workflow is something that comes embedded in the auth token
// so it can be used to make sure the user is not spoofing a different run that doesn't have access to
func (uc *AttestationStateUseCase) checkWorkflowRunInWorkflow(ctx context.Context, workflowID, runID string) (*uuid.UUID, error) {
	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	run, err := uc.wfRunRepo.FindByID(ctx, runUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to workflow run: %w", err)
	} else if run == nil {
		return nil, NewErrNotFound("workflow run")
	}

	if run.Workflow.ID != workflowUUID {
		return nil, NewErrNotFound("workflow run")
	}

	return &runUUID, nil
}

// The following code is in charge of symmetric encryption and decryption of the attestation state
// The only purpose is to have encryption at rest in the database
const (
	saltSize   = 16
	iterations = 10000
	keySize    = 32 // AES-256
	// The magic string is used to check if the passphrase is correct
	// It's prepended to the plaintext before encryption
	// If the passphrase is incorrect, the decrypted data won't start with this string
	magic = "MagicPrefix__"
	// 6MB limit to protect against allocation overflows
	maxEncryptedSize = 6 * 1024 * 1024
)

// Generate an AES key derived from the passphrase and a salt
func generateKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, iterations, keySize, sha256.New)
}

func encrypt(data []byte, passphrase string) ([]byte, error) {
	// 6MB limit to protect against allocation overflows
	if len(data) > maxEncryptedSize {
		return nil, errors.New("value too large")
	}

	// Prepend magic string to the plaintext
	plaintextWithMagic := append([]byte(magic), data...)
	ciphertext := make([]byte, aes.BlockSize+len(plaintextWithMagic))

	// generate salt
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	// generate iv
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// Derive the key from the passphrase and the salt
	key := generateKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// perform encryption
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintextWithMagic)

	return append(salt, ciphertext...), nil
}

func decrypt(ciphertext []byte, passphrase string) ([]byte, error) {
	if len(ciphertext) <= saltSize {
		return nil, fmt.Errorf("encrypted data too short")
	}

	salt := ciphertext[:saltSize]
	ciphertext = ciphertext[saltSize:]

	key := generateKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	// Check if the decrypted data starts with the magic string
	if string(ciphertext[:len(magic)]) != magic {
		return nil, fmt.Errorf("incorrect passphrase")
	}

	return ciphertext[len(magic):], nil
}
