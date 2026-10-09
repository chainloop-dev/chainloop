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

package pi

import (
	"encoding/json"
	"time"
)

const (
	sessionVersion       = 3
	treeMarkerCustomType = "chainloop-trace-tree-marker"
)

type sessionHeader struct {
	Type      string `json:"type"`
	Version   int    `json:"version"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
}

type sessionEntry struct {
	Type       string
	ID         string
	ParentID   string
	HasParent  bool
	Timestamp  string
	CustomType string
	Message    json.RawMessage
	Raw        json.RawMessage
	Line       int
}

type parsedSession struct {
	Header        sessionHeader
	StartedAt     time.Time
	Selected      []*sessionEntry
	TruncatedLine int
}
