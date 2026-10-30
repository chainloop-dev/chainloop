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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Inference limits. A response is untrusted input, so it is bounded in bytes
// and time.
const (
	// InferenceTimeout bounds one chat request.
	InferenceTimeout = 60 * time.Second
	// MaxInferenceBytes bounds one chat response.
	MaxInferenceBytes = 4 << 20
)

// Message is one chat message in the OpenAI wire format.
type Message struct {
	// Role is system, user, or assistant.
	Role string `json:"role"`
	// Content is the text of the message.
	Content string `json:"content"`
}

// ChatRequest is a minimal OpenAI-compatible chat request.
type ChatRequest struct {
	// Model is a model ID from the catalog, with its namespace kept.
	Model string `json:"model"`
	// Messages are the conversation so far.
	Messages []Message `json:"messages"`
	// MaxTokens optionally bounds the answer.
	MaxTokens int `json:"max_tokens,omitempty"`
}

// chatResponse is the part of the response the client reads.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Client calls the OrcaRouter inference API with a credential, whatever adapter
// produced it.
type Client struct {
	// BaseURL is the inference base URL. Empty uses APIOrigin().
	BaseURL string
	// HTTP is the HTTP client. Empty uses one bounded by InferenceTimeout.
	HTTP *http.Client
}

// Chat sends a chat request. A rejected credential returns ErrNeedsReauth; the
// caller marks the exact generation that made the request, and does not retry
// or attempt a refresh, because the credential is durable rather than
// refreshable.
func (c *Client) Chat(ctx context.Context, cred Credential, req ChatRequest) (string, error) {
	if cred.Key == "" {
		return "", ErrNoCredential
	}
	if strings.TrimSpace(req.Model) == "" {
		return "", errors.New("no model selected")
	}
	if len(req.Messages) == 0 {
		return "", errors.New("no messages")
	}

	base := c.BaseURL
	if base == "" {
		base = APIOrigin()
	}
	base = NormalizeBase(base)
	if err := ValidateOrigin(base); err != nil {
		return "", err
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("encode the chat request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, InferenceTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build the chat request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+cred.Key)
	httpReq.Header.Set("Content-Type", "application/json")

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: InferenceTimeout}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send the chat request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxInferenceBytes))
	if err != nil {
		return "", fmt.Errorf("read the chat response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusUnauthorized:
		return "", ErrNeedsReauth
	case resp.StatusCode == http.StatusForbidden:
		// The key is valid but is not allowed to use this model. Switching
		// models or granting access is the fix; a new login would not help.
		return "", fmt.Errorf("the credential is not allowed to use model %q; list the models it can use with `chainloop orcarouter models`, or grant access to it in the OrcaRouter console (status 403)", req.Model)
	case resp.StatusCode == http.StatusTooManyRequests:
		return "", errors.New("OrcaRouter rate limited the request (status 429)")
	default:
		return "", fmt.Errorf("chat request failed: status %d", resp.StatusCode)
	}

	var decoded chatResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("decode the chat response: %w", err)
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return "", fmt.Errorf("chat request failed: %s", decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("the chat response has no choices")
	}
	return decoded.Choices[0].Message.Content, nil
}
