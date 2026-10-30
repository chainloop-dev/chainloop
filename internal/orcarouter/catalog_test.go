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
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureCatalog covers one record of each capability the filters must tell
// apart: text-only chat, image-input chat, embedding, image generation, video,
// and rerank.
const fixtureCatalog = `{
  "object": "list",
  "data": [
    {"id":"orcarouter/auto","owned_by":"OrcaRouter","supported_endpoint_types":["openai","anthropic"]},
    {"id":"openai/gpt-5.5","owned_by":"OpenAI","supported_endpoint_types":["openai","openai-response"],
     "architecture":{"input_modalities":["text","image"],"output_modalities":["text"]},
     "reasoning":{"efforts":["low","medium","high","xhigh"]}},
    {"id":"deepseek/deepseek-v4-pro","owned_by":"DeepSeek","supported_endpoint_types":["openai","openai-response"],
     "architecture":{"input_modalities":["text"],"output_modalities":["text"]}},
    {"id":"vendor/no-modalities","owned_by":"Vendor","supported_endpoint_types":["openai"]},
    {"id":"vendor/image-gen","owned_by":"Vendor","supported_endpoint_types":["image-generation"]},
    {"id":"vendor/video","owned_by":"Vendor","supported_endpoint_types":["openai-video"]},
    {"id":"vendor/rerank","owned_by":"Vendor","supported_endpoint_types":["jina-rerank"]},
    {"id":"vendor/embed","owned_by":"Vendor","supported_endpoint_types":["embeddings"]},
    {"id":"orca/orcacyber-ultra-1.0","owned_by":"OrcaRouter","supported_endpoint_types":["openai"],
     "architecture":{"input_modalities":["text"],"output_modalities":["text"]}}
  ]
}`

func catalogServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/models", r.URL.Path)
		assert.Equal(t, "Bearer sk-orca-fixture", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDiscoverCatalogLive(t *testing.T) {
	srv := catalogServer(t, fixtureCatalog, http.StatusOK)
	cred := Credential{Key: "sk-orca-fixture"}

	cat := DiscoverCatalog(context.Background(), srv.Client(), srv.URL, cred)
	require.False(t, cat.Degraded)
	assert.Equal(t, SourceLive, cat.Source)
	require.NoError(t, cat.Err)
	assert.Len(t, cat.Models, 9)
	assert.True(t, Contains(cat.Models, "openai/gpt-5.5"))
	// The live result is authoritative: the seed is not merged into it.
	assert.False(t, Contains(cat.Models, "anthropic/claude-opus-4.8"))
}

func TestDiscoverCatalogFallsBack(t *testing.T) {
	// A server that refuses the catalog: a fresh installation still gets a
	// usable, clearly degraded list.
	srv := catalogServer(t, `{}`, http.StatusInternalServerError)
	cred := Credential{Key: "sk-orca-fixture"}

	cat := DiscoverCatalog(context.Background(), srv.Client(), srv.URL, cred)
	assert.True(t, cat.Degraded)
	assert.Equal(t, SourceSeed, cat.Source)
	assert.Error(t, cat.Err)
	assert.NotEmpty(t, cat.Models)

	// The verified metadata survives the fallback.
	gpt := findModel(t, cat.Models, "openai/gpt-5.5")
	assert.Equal(t, []string{"text", "image"}, gpt.InputModalities)
	assert.Equal(t, []string{"low", "medium", "high", "xhigh"}, gpt.ReasoningEfforts)
}

func TestDiscoverCatalogUnauthorized(t *testing.T) {
	srv := catalogServer(t, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	cat := DiscoverCatalog(context.Background(), srv.Client(), srv.URL, Credential{Key: "sk-orca-fixture"})
	assert.True(t, cat.Degraded)
	assert.True(t, errors.Is(cat.Err, ErrNeedsReauth))
}

func TestDiscoverCatalogNoCredential(t *testing.T) {
	cat := DiscoverCatalog(context.Background(), nil, "https://api.orcarouter.ai/v1", Credential{})
	assert.True(t, cat.Degraded)
	assert.Equal(t, SourceSeed, cat.Source)
}

func TestChatModelsFilter(t *testing.T) {
	cat := parseFixture(t)

	textIDs := ids(ChatModels(cat, nil))
	assert.Contains(t, textIDs, "orcarouter/auto")
	assert.Contains(t, textIDs, "deepseek/deepseek-v4-pro")
	assert.Contains(t, textIDs, "vendor/no-modalities", "a chat record with no modality block is still text chat")
	assert.NotContains(t, textIDs, "vendor/image-gen")
	assert.NotContains(t, textIDs, "vendor/video")
	assert.NotContains(t, textIDs, "vendor/rerank")
	assert.NotContains(t, textIDs, "vendor/embed")

	imageModels := ChatModels(cat, []string{"image"})
	imageIDs := ids(imageModels)
	assert.Contains(t, imageIDs, "openai/gpt-5.5")
	assert.NotContains(t, imageIDs, "deepseek/deepseek-v4-pro", "a text-only model must leave an image dropdown")
	assert.NotContains(t, imageIDs, "orcarouter/auto", "an undeclared modality fails closed")
	assert.NotContains(t, imageIDs, "vendor/no-modalities")
}

func TestCapabilityFilters(t *testing.T) {
	cat := parseFixture(t)

	cases := []struct {
		capability Capability
		want       string
	}{
		{capability: CapabilityEmbedding, want: "vendor/embed"},
		{capability: CapabilityImage, want: "vendor/image-gen"},
		{capability: CapabilityVideo, want: "vendor/video"},
		{capability: CapabilityRerank, want: "vendor/rerank"},
	}
	for _, tc := range cases {
		t.Run(string(tc.capability), func(t *testing.T) {
			got := ids(ForCapability(cat, tc.capability, nil))
			assert.Equal(t, []string{tc.want}, got)
		})
	}
}

func TestRevalidate(t *testing.T) {
	cat := parseFixture(t)
	text := ChatModels(cat, nil)

	assert.Equal(t, "deepseek/deepseek-v4-pro", Revalidate(text, "deepseek/deepseek-v4-pro"))
	assert.Empty(t, Revalidate(text, "vendor/image-gen"), "a stored model that no longer fits is cleared")
	assert.Empty(t, Revalidate(text, ""))

	image := ChatModels(cat, []string{"image"})
	assert.Empty(t, Revalidate(image, "deepseek/deepseek-v4-pro"), "adding an image attachment invalidates a text model")
}

func TestCatalogBounds(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"data":[`)
	for i := 0; i < MaxCatalogItems+50; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"id":"vendor/m`)
		b.WriteString(strings.Repeat("x", 1))
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`","supported_endpoint_types":["openai"]}`)
	}
	b.WriteString(`]}`)

	srv := catalogServer(t, b.String(), http.StatusOK)
	cat := DiscoverCatalog(context.Background(), srv.Client(), srv.URL, Credential{Key: "sk-orca-fixture"})
	assert.Len(t, cat.Models, MaxCatalogItems)
}

func TestCatalogRejectsMalformedRecords(t *testing.T) {
	body := `{"data":[{"id":""},{"nosuchfield":"x"},{"id":"vendor/ok","supported_endpoint_types":["openai"]}]}`
	srv := catalogServer(t, body, http.StatusOK)
	cat := DiscoverCatalog(context.Background(), srv.Client(), srv.URL, Credential{Key: "sk-orca-fixture"})
	require.Len(t, cat.Models, 1)
	assert.Equal(t, "vendor/ok", cat.Models[0].ID)
}

func parseFixture(t *testing.T) []Model {
	t.Helper()
	srv := catalogServer(t, fixtureCatalog, http.StatusOK)
	cat := DiscoverCatalog(context.Background(), srv.Client(), srv.URL, Credential{Key: "sk-orca-fixture"})
	require.False(t, cat.Degraded)
	return cat.Models
}

func ids(models []Model) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		out = append(out, m.ID)
	}
	return out
}

func findModel(t *testing.T, models []Model, id string) Model {
	t.Helper()
	i := slices.IndexFunc(models, func(m Model) bool { return m.ID == id })
	require.GreaterOrEqual(t, i, 0)
	return models[i]
}
