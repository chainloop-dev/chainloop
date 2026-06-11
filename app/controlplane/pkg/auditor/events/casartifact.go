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

package events

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/google/uuid"
)

var (
	_ auditor.LogEntry = (*CASArtifactUploaded)(nil)
	_ auditor.LogEntry = (*CASArtifactDownloaded)(nil)
)

const (
	CASArtifactType                 auditor.TargetType = "CASArtifact"
	CASArtifactUploadedActionType   string             = "CASArtifactUploaded"
	CASArtifactDownloadedActionType string             = "CASArtifactDownloaded"
)

// CASArtifactBase contains the common fields for all CAS artifact events.
// These events are emitted by the Artifact CAS data plane, where only the
// organization (not the user) is known, so they don't require an actor.
type CASArtifactBase struct {
	// Digest is the sha256 hex digest of the artifact
	Digest string `json:"digest"`
	// SizeBytes is the size of the artifact, 0 when unknown
	SizeBytes   int64  `json:"size_bytes"`
	FileName    string `json:"file_name,omitempty"`
	BackendType string `json:"backend_type,omitempty"`
}

func (c *CASArtifactBase) RequiresActor() bool {
	return false
}

func (c *CASArtifactBase) TargetType() auditor.TargetType {
	return CASArtifactType
}

// TargetID is nil since artifacts are identified by their digest, carried in the action info
func (c *CASArtifactBase) TargetID() *uuid.UUID {
	return nil
}

func (c *CASArtifactBase) validate() error {
	if c.Digest == "" {
		return errors.New("digest is required")
	}

	return nil
}

// CASArtifactUploaded represents an artifact upload to the CAS.
// Skipped is true when the upload was deduplicated: the artifact already
// existed in the backend and no bytes were transferred or stored.
type CASArtifactUploaded struct {
	*CASArtifactBase
	Skipped bool `json:"skipped"`
}

func (c *CASArtifactUploaded) ActionType() string {
	return CASArtifactUploadedActionType
}

func (c *CASArtifactUploaded) ActionInfo() (json.RawMessage, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	return json.Marshal(&c)
}

func (c *CASArtifactUploaded) Description() string {
	if c.Skipped {
		return fmt.Sprintf("upload of artifact %s skipped, already exists", c.Digest)
	}

	return fmt.Sprintf("artifact %s (%d bytes) was uploaded", c.Digest, c.SizeBytes)
}

// CASArtifactDownloaded represents an artifact download from the CAS
type CASArtifactDownloaded struct {
	*CASArtifactBase
}

func (c *CASArtifactDownloaded) ActionType() string {
	return CASArtifactDownloadedActionType
}

func (c *CASArtifactDownloaded) ActionInfo() (json.RawMessage, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	return json.Marshal(&c)
}

func (c *CASArtifactDownloaded) Description() string {
	return fmt.Sprintf("artifact %s (%d bytes) was downloaded", c.Digest, c.SizeBytes)
}
