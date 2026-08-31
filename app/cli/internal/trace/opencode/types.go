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

package opencode

import "encoding/json"

// types.go mirrors the JSON structures emitted by `opencode export <sessionID>`
// and `opencode session list --format json`. Only the fields consumed by the
// trace provider are modelled; unknown fields are ignored by encoding/json.

// exportData is the top-level shape of `opencode export`:
//
//	{ "info": SessionInfo, "messages": [{ "info": MessageInfo, "parts": [Part] }] }
type exportData struct {
	Info     sessionInfo    `json:"info"`
	Messages []messageEntry `json:"messages"`
}

// sessionInfo mirrors SessionV1.SessionInfo in the opencode schema.
type sessionInfo struct {
	ID        string      `json:"id"`
	Slug      string      `json:"slug"`
	Title     string      `json:"title"`
	Directory string      `json:"directory"`
	Version   string      `json:"version"`
	Cost      *float64    `json:"cost,omitempty"`
	Tokens    *sessTokens `json:"tokens,omitempty"`
	Time      sessTime    `json:"time"`
}

type sessTime struct {
	Created float64 `json:"created"`
	Updated float64 `json:"updated"`
}

type sessTokens struct {
	Input  int            `json:"input"`
	Output int            `json:"output"`
	Cache  sessTokenCache `json:"cache"`
}

type sessTokenCache struct {
	Read  int `json:"read"`
	Write int `json:"write"`
}

// messageEntry pairs a message's metadata with its parts.
type messageEntry struct {
	Info  messageInfo `json:"info"`
	Parts []part      `json:"parts"`
}

// messageInfo is the union of User and Assistant message metadata. The
// discriminator is the Role field. Only fields we consume are modelled.
type messageInfo struct {
	Role       string         `json:"role"`
	ID         string         `json:"id"`
	ModelID    string         `json:"modelID,omitempty"`
	ProviderID string         `json:"providerID,omitempty"`
	Cost       float64        `json:"cost,omitempty"`
	Tokens     *msgTokens     `json:"tokens,omitempty"`
	Time       *msgTime       `json:"time,omitempty"`
	Model      *userModelInfo `json:"model,omitempty"`
}

type userModelInfo struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

type msgTokens struct {
	Input  int           `json:"input"`
	Output int           `json:"output"`
	Cache  msgTokenCache `json:"cache"`
}

type msgTokenCache struct {
	Read  int `json:"read"`
	Write int `json:"write"`
}

type msgTime struct {
	Created   float64 `json:"created"`
	Completed float64 `json:"completed,omitempty"`
}

// part is a union of all part types in the opencode schema. The
// discriminator is the Type field. Only fields we consume are modelled.
type part struct {
	Type  string     `json:"type"`
	ID    string     `json:"id,omitempty"`
	Text  string     `json:"text,omitempty"`
	Tool  string     `json:"tool,omitempty"`
	State *toolState `json:"state,omitempty"`
}

// toolState is the union of tool states. The discriminator is the Status
// field. Input and Output are captured as raw JSON so the frontend can
// display tool call details without us modelling every tool's schema.
type toolState struct {
	Status string          `json:"status"`
	Input  json.RawMessage `json:"input,omitempty"`
	Output json.RawMessage `json:"output,omitempty"`
}

// sessionListEntry mirrors one element of `opencode session list --format json`.
type sessionListEntry struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Directory string  `json:"directory"`
	Updated   float64 `json:"updated"`
	Created   float64 `json:"created"`
}
