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

package filesystem

import (
	"fmt"
	"io"
	"os"

	v1 "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/statemanager"
	"google.golang.org/protobuf/encoding/protojson"
)

type Filesystem struct {
	statePath string
}

func New(statePath string) (*Filesystem, error) {
	if statePath == "" {
		return nil, fmt.Errorf("state path cannot be empty")
	}

	return &Filesystem{statePath}, nil
}

func (l *Filesystem) String() string {
	return fmt.Sprintf("file://%s", l.statePath)
}

func (l *Filesystem) Initialized() (bool, error) {
	if file, err := os.Stat(l.statePath); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to check state file: %w", err)
	} else if file != nil {
		return true, nil
	}

	return false, nil
}

func (l *Filesystem) Write(state *v1.CraftingState) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}

	marshaler := protojson.MarshalOptions{Indent: "  "}
	raw, err := marshaler.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Create empty file
	file, err := os.Create(l.statePath)
	if err != nil {
		return fmt.Errorf("failed to create state file: %w", err)
	}
	defer file.Close()

	_, err = file.Write(raw)
	if err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

func (l *Filesystem) Read(state *v1.CraftingState) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}

	file, err := os.Open(l.statePath)
	if err != nil && os.IsNotExist(err) {
		return &statemanager.ErrNotFound{Path: l.statePath}
	} else if err != nil {
		return fmt.Errorf("failed to open state file: %w", err)
	}
	defer file.Close()

	stateRaw, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	if err := protojson.Unmarshal(stateRaw, state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return nil
}

func (l *Filesystem) Reset() error {
	if err := os.Remove(l.statePath); err != nil && os.IsNotExist(err) {
		return &statemanager.ErrNotFound{Path: l.statePath}
	} else if err != nil {
		return fmt.Errorf("failed to remove state file: %w", err)
	}

	return nil
}
