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

package local

import (
	"fmt"
	"io"
	"os"

	v1 "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type Local struct {
	statePath string
}

func New(statePath string) (*Local, error) {
	if statePath == "" {
		return nil, fmt.Errorf("state path cannot be empty")
	}

	return &Local{statePath}, nil
}

func (l *Local) Info() string {
	return l.statePath
}

func (l *Local) Initialized() (bool, error) {
	if file, err := os.Stat(l.statePath); err != nil {
		return false, fmt.Errorf("failed to check state file: %w", err)
	} else if file != nil {
		return true, nil
	}

	return false, nil
}

func (l *Local) Write(state *v1.CraftingState) error {
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

func (l *Local) Read(state *v1.CraftingState) error {
	file, err := os.Open(l.statePath)
	if err != nil {
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

func (l *Local) Reset() error {
	return os.Remove(l.statePath)
}
