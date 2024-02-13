// Copyright 2024 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	// https://github.com/chainloop-dev/chainloop/releases/tag/v0.60.0
	clImage = "ghcr.io/chainloop-dev/chainloop/cli@sha256:4e0bc402f71f4877a1ae8d6df5eb4e666a0efa0e7d43ab4f97f21c0e46ae0a59"
)

type Chainloop struct {
	Token *Secret
}

func New(token string) *Chainloop {
	return &Chainloop{
		dag.SetSecret("CHAINLOOP_TOKEN", token),
	}
}

// Start the attestation crafting process
func (m *Chainloop) AttestationInit(ctx context.Context) (string, error) {
	info, err := m.cliImage().WithExec([]string{"attestation", "init", "--remote-state", "-o", "json"}).Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("running attestation init: %w", err)
	}

	resp := struct {
		AttestationID string `json:"AttestationID"`
	}{}

	if err := json.Unmarshal([]byte(info), &resp); err != nil {
		return "", fmt.Errorf("unmarshalling attestation init response: %w", err)
	}

	return resp.AttestationID, nil
}

// Check the status of the current attestation
func (m *Chainloop) AttestationStatus(ctx context.Context, attestationID string) (string, error) {
	return m.cliImage().WithExec([]string{
		"attestation", "status",
		"--remote-state",
		"--attestation-id", attestationID,
	}).Stdout(ctx)
}

// Add a piece of evidence/material to the current attestation
func (m *Chainloop) AttestationAdd(ctx context.Context, name string, value *File, attestationID string) (string, error) {
	fileName, err := value.Name(ctx)
	if err != nil {
		return "", fmt.Errorf("getting file name: %w", err)
	}

	filePath := fmt.Sprintf("/tmp/%s", fileName)

	return m.cliImage().
		WithFile(filePath, value).
		WithExec([]string{
			"attestation", "add",
			"--remote-state",
			"--attestation-id", attestationID,
			"--name", name,
			"--value", filePath,
		}).Stderr(ctx)
}

// Generate, sign and push the attestation to the control plane
func (m *Chainloop) AttestationPush(ctx context.Context, signingKey *File, passphrase, attestationID string) (string, error) {
	return m.cliImage().
		WithFile("/tmp/key.pem", signingKey).
		WithSecretVariable("CHAINLOOP_SIGNING_PASSWORD", dag.SetSecret("CL", passphrase)).
		WithExec([]string{
			"attestation", "push",
			"--remote-state",
			"--attestation-id", attestationID,
			"--key", "/tmp/key.pem",
		}).Stdout(ctx)
}

// Mark current attestation process as canceled or failed. --trigger = "failure" | "cancellation" (default: "failure")
func (m *Chainloop) AttestationReset(ctx context.Context, trigger, reason Optional[string], attestationID string) (string, error) {
	opts := []string{
		"attestation", "reset",
		"--remote-state",
		"--attestation-id", attestationID,
	}

	if reason.isSet {
		opts = append(opts, "--reason", reason.value)
	}

	if trigger.isSet {
		opts = append(opts, "--trigger", trigger.value)
	}

	return m.cliImage().WithExec(opts).Stdout(ctx)
}

func (m *Chainloop) cliImage() *Container {
	return dag.Container().
		From(clImage).
		WithSecretVariable("CHAINLOOP_ROBOT_ACCOUNT", m.Token).
		WithEnvVariable("CACHEBUSTER", time.Now().String())
}
