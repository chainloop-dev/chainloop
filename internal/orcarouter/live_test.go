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
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// liveModelEnv selects the model of the live check. It defaults to the
// verification model this repository's key can call; the provider's own
// recommended models are listed in RecommendedModels.
const (
	liveModelEnv     = "ORCAROUTER_LIVE_MODEL"
	liveModelDefault = "deepseek/deepseek-v4.1-flash"
	livePrompt       = "Reply with the single word: ok"
)

// TestLiveCatalogAndChat exercises the real provider through the code paths
// this package ships: the credential adapter, the catalog request, the
// capability filter, and the chat request. It is skipped unless a key is in
// the environment, so the offline suite stays hermetic. It never prints the
// key.
func TestLiveCatalogAndChat(t *testing.T) {
	key := os.Getenv(EnvKey)
	if key == "" {
		t.Skipf("%s is not set; skipping the live check", EnvKey)
	}

	manager, err := NewManager(&MemoryStore{})
	require.NoError(t, err)
	cred, err := manager.SetAPIKey(key)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. The live catalog comes from the configured origin, not a fixture.
	cat := DiscoverCatalog(ctx, nil, APIOrigin(), cred)
	require.False(t, cat.Degraded, "live discovery failed: %v", cat.Err)
	require.Equal(t, SourceLive, cat.Source)
	require.NotEmpty(t, cat.Models)
	t.Logf("live catalog: %d models", len(cat.Models))

	// 2. The chat filter yields only text models, and the model under test is
	// among them.
	chat := ChatModels(cat.Models, nil)
	require.NotEmpty(t, chat)
	for _, model := range chat {
		assert.True(t, model.IsTextChat(), model.ID)
	}

	model := os.Getenv(liveModelEnv)
	if model == "" {
		model = liveModelDefault
	}
	require.True(t, Contains(chat, model), "model %q is not in the filtered chat catalog", model)
	assert.Equal(t, model, Revalidate(chat, model))

	// 3. A real request through the client, with the credential the adapter
	// produced.
	client := &Client{}
	answer, err := client.Chat(ctx, cred, ChatRequest{
		Model:    model,
		Messages: []Message{{Role: "user", Content: livePrompt}},
	})
	require.NoError(t, err)
	answer = strings.TrimSpace(answer)
	assert.NotEmpty(t, answer)
	t.Logf("live chat with %s returned %d characters", model, len(answer))

	// 4. The image-input filter is a strict subset of the chat list.
	withImage := ChatModels(cat.Models, []string{"image"})
	assert.LessOrEqual(t, len(withImage), len(chat))
	for _, model := range withImage {
		assert.True(t, model.DeclaresInput("image"), model.ID)
	}
	t.Logf("live catalog: %d image-input chat models", len(withImage))
}
