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

package codex

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	responseIDKey  = "response_id"
	usageKey       = "usage"
	threadUsageKey = "thread_token_usage"
)

func parseUsageFixture(t *testing.T, records string) *aicodingsession.Data {
	t.Helper()
	dir := t.TempDir()
	header := `{"timestamp":"2026-10-22T00:00:00Z","type":"session_meta","payload":{"id":"sess-1","model_provider":"openai"}}` + "\n"
	header += `{"timestamp":"2026-10-22T00:00:00Z","type":"turn_context","payload":{"model":"gpt-6.1-sol"}}` + "\n"
	require.NoError(t, os.WriteFile(state.RawSessionPath(dir, testSessionID), []byte(header+records), 0600)) //nolint:gosec // temporary fixture path and a fixed session ID
	evidence, err := New().ParseSession(t.Context(), &trace.ParseOpts{SessionDir: dir, SessionID: testSessionID})
	require.NoError(t, err)
	return &evidence.Data
}

func usageJSONL(t *testing.T, payloads ...map[string]any) string {
	t.Helper()
	var result strings.Builder
	for _, payload := range payloads {
		kind := "token_usage_record"
		if payload["type"] == "token_count" {
			kind = "event_msg"
		}
		raw, err := json.Marshal(map[string]any{"timestamp": "2026-10-22T00:00:01Z", "type": kind, "payload": payload})
		require.NoError(t, err)
		result.Write(raw)
		result.WriteByte('\n')
	}
	return result.String()
}

func TestUsageSourceSelection(t *testing.T) {
	first := tokenUsage{Input: 1000, Cached: 800, CacheWrite: 100, Output: 50, Total: 1050}
	second := tokenUsage{Input: 2000, Cached: 1600, CacheWrite: 200, Output: 100, Total: 2100}
	response := func(id string, usage, total tokenUsage) map[string]any {
		return map[string]any{responseIDKey: id, usageKey: usage, threadUsageKey: total}
	}
	native := func(total tokenUsage) map[string]any {
		return map[string]any{threadUsageKey: total}
	}
	legacy := func(total tokenUsage) map[string]any {
		return map[string]any{"type": "token_count", "info": map[string]any{"total_token_usage": total}}
	}
	for _, tc := range []struct {
		name     string
		records  []map[string]any
		warnings int
	}{
		{
			name: "responses survive compaction resets and duplicate replay",
			records: []map[string]any{
				legacy(second), response("a", first, first), legacy(first),
				// Both cumulative streams can reset without changing the
				// identity or usage of a new response in a resumed session.
				response("b", first, first), legacy(second),
				response("a", first, first), response("b", first, second),
			},
		},
		{
			name: "native counters take precedence over legacy snapshots",
			records: []map[string]any{
				legacy(second), native(first), legacy(first), native(second), legacy(first),
			},
		},
		{
			name: "partial response records fall back to native counters",
			records: []map[string]any{
				response("a", first, first), native(second), legacy(first),
			},
		},
		{
			name: "native counters ignore repeats and stale snapshots",
			records: []map[string]any{
				native(first), native(second), native(second), native(first), native(second),
			},
			warnings: 1,
		},
		{
			name: "legacy counters ignore repeats and stale snapshots",
			records: []map[string]any{
				legacy(first), legacy(second), legacy(second), legacy(first), legacy(second),
			},
			warnings: 1,
		},
		{
			name: "cumulative snapshots with the same response ID still advance",
			records: []map[string]any{
				{responseIDKey: "a", threadUsageKey: first},
				{responseIDKey: "a", threadUsageKey: second},
			},
		},
		{
			name:     "native counters end on a stale snapshot",
			records:  []map[string]any{native(first), native(second), native(first)},
			warnings: 1,
		},
		{
			name:     "legacy counters end on a stale snapshot",
			records:  []map[string]any{legacy(first), legacy(second), legacy(first)},
			warnings: 1,
		},
		{
			name: "response usage does not require cumulative counters",
			records: []map[string]any{
				{responseIDKey: "a", usageKey: first}, {responseIDKey: "b", usageKey: first},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := parseUsageFixture(t, usageJSONL(t, tc.records...))
			require.NotNil(t, d.Usage)
			assert.Equal(t, 200, d.Usage.InputTokens)
			assert.Equal(t, 100, d.Usage.OutputTokens)
			assert.Equal(t, 300, d.Usage.TotalTokens)
			assert.Equal(t, 1600, d.Usage.CacheReadInputTokens)
			assert.Equal(t, 200, d.Usage.CacheCreationInputTokens)
			assert.InDelta(t, 0.0021, d.Usage.EstimatedCostUSD, 0.00001)
			assert.Len(t, d.Warnings, tc.warnings)
			if tc.warnings != 0 {
				assert.Contains(t, d.Warnings[0], "ignoring snapshot")
			}
		})
	}
}

func TestCompactedSessionUsageRegression(t *testing.T) {
	raw, err := os.ReadFile("testdata/compacted-session.jsonl")
	require.NoError(t, err)
	d := parseUsageFixture(t, string(raw))
	require.NotNil(t, d.Usage)
	assert.Empty(t, d.Warnings)
	assert.Equal(t, 20425, d.Usage.InputTokens)
	assert.Equal(t, 51113536, d.Usage.CacheReadInputTokens)
	assert.Equal(t, 669175, d.Usage.CacheCreationInputTokens)
	assert.Equal(t, 168426, d.Usage.OutputTokens)
	assert.Equal(t, 188851, d.Usage.TotalTokens)
	// Reconstruct Codex's inclusive totals from the disjoint evidence fields.
	input := d.Usage.InputTokens + d.Usage.CacheReadInputTokens + d.Usage.CacheCreationInputTokens
	assert.Equal(t, 51803136, input)
	assert.Equal(t, 51971562, input+d.Usage.OutputTokens)
	assert.InDelta(t, 8.5094, d.Usage.EstimatedCostUSD, 0.00001)
}

func TestResponseUsageAcrossModelsAndResume(t *testing.T) {
	response := map[string]any{
		"response_id": "a",
		usageKey:      tokenUsage{Input: 1000, Cached: 800, CacheWrite: 100, Output: 50, Total: 1050},
	}
	records := usageJSONL(t, response)
	// A resumed session repeats its metadata and an old response after a model
	// switch. The old response must not be billed again under the new model.
	records += `{"timestamp":"2026-10-22T00:00:02Z","type":"session_meta","payload":{"id":"sess-1","model_provider":"openai"}}` + "\n"
	records += `{"timestamp":"2026-10-22T00:00:02Z","type":"turn_context","payload":{"model":"gpt-5-mini"}}` + "\n"
	records += usageJSONL(t, response)
	response[responseIDKey] = "b"
	records += usageJSONL(t, response)
	d := parseUsageFixture(t, records)
	assert.Empty(t, d.Warnings)
	assert.Equal(t, 200, d.Usage.InputTokens)
	assert.Equal(t, 100, d.Usage.OutputTokens)
	assert.Equal(t, 300, d.Usage.TotalTokens)
	assert.Equal(t, 1600, d.Usage.CacheReadInputTokens)
	assert.Equal(t, 200, d.Usage.CacheCreationInputTokens)
	assert.InDelta(t, 0.0012, d.Usage.EstimatedCostUSD, 0.00001)
}

func TestResponseUsageValidation(t *testing.T) {
	for _, tc := range []struct {
		name, usage, warning string
	}{
		{"negative", `{"input_tokens":-1}`, "negative Codex token count"},
		{"cache read exceeds input", `{"input_tokens":100,"cached_input_tokens":101}`, "cache tokens exceed input"},
		{"cache write exceeds remaining input", `{"input_tokens":100,"cached_input_tokens":90,"cache_write_input_tokens":11}`, "cache tokens exceed input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := parseUsageFixture(t, `{"timestamp":"2026-10-22T00:00:01Z","type":"token_usage_record","payload":{"response_id":"a","usage":`+tc.usage+`}}`)
			assert.Nil(t, d.Usage)
			assert.Contains(t, strings.Join(d.Warnings, "\n"), tc.warning)
		})
	}
}

func TestOutputIncludesReasoningOnce(t *testing.T) {
	d := parseUsageFixture(t, `{"timestamp":"2026-10-22T00:00:01Z","type":"token_usage_record","payload":{"response_id":"a","usage":{"input_tokens":1000,"cached_input_tokens":800,"cache_write_input_tokens":100,"output_tokens":50,"reasoning_output_tokens":40,"total_tokens":1050}}}`)
	assert.Empty(t, d.Warnings)
	assert.Equal(t, 100, d.Usage.InputTokens)
	assert.Equal(t, 50, d.Usage.OutputTokens)
	assert.Equal(t, 150, d.Usage.TotalTokens)
	assert.InDelta(t, 0.001, d.Usage.EstimatedCostUSD, 0.00001)
}
