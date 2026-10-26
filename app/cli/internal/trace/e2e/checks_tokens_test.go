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

package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/codex"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const codexCacheRecord = `{"type":"token_usage_record","payload":{"response_id":"a","usage":{"input_tokens":1000,"cached_input_tokens":800,"cache_write_input_tokens":100,"output_tokens":50,"reasoning_output_tokens":40}}}`

const tokenMainTranscript = "main"

func cacheSession(agent string, records ...string) *Session {
	s := &Session{Stored: &StoredSession{}}
	d := &s.Stored.Evidence.Data
	d.Agent.Name = agent
	d.SchemaVersion = "v1"
	if agent == codex.Name {
		d.SchemaVersion = "v2"
	}
	d.Usage = &aicodingsession.Usage{
		InputTokens: 100, OutputTokens: 50, TotalTokens: 150,
		CacheReadInputTokens: 800, CacheCreationInputTokens: 100, EstimatedCostUSD: 0.001,
	}
	d.RawSession = map[string][]json.RawMessage{tokenMainTranscript: {}}
	for _, r := range records {
		d.RawSession[tokenMainTranscript] = append(d.RawSession[tokenMainTranscript], json.RawMessage(r))
	}
	return s
}

func TestCheckCacheTokenAccounting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*aicodingsession.Usage)
	}{
		{name: "base input and separate cache"},
		{name: "missing cache reads", change: func(u *aicodingsession.Usage) { u.CacheReadInputTokens = 0 }},
		{name: "missing cache writes", change: func(u *aicodingsession.Usage) { u.CacheCreationInputTokens = 0 }},
		{name: "cache included in input", change: func(u *aicodingsession.Usage) { u.InputTokens = 1000; u.TotalTokens = 1050 }},
		{name: "cache included in total", change: func(u *aicodingsession.Usage) { u.TotalTokens = 1050 }},
		{name: "reasoning added twice", change: func(u *aicodingsession.Usage) { u.OutputTokens = 90; u.TotalTokens = 190 }},
		{name: "self consistent inflated counts", change: func(u *aicodingsession.Usage) {
			u.InputTokens *= 101
			u.OutputTokens *= 101
			u.TotalTokens *= 101
			u.CacheReadInputTokens *= 101
			u.CacheCreationInputTokens *= 101
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := cacheSession(codex.Name, codexCacheRecord, codexCacheRecord)
			if tc.change == nil {
				require.NoError(t, checkCacheTokenAccounting(s))
				return
			}
			tc.change(evidence(s).Usage)
			err := checkCacheTokenAccounting(s)
			require.Error(t, err)
			assert.False(t, errors.Is(err, errNotCovered))
		})
	}
}

func TestCheckClaudeCacheAccountingIncludesSubagents(t *testing.T) {
	raw := `{"type":"assistant","message":{"usage":{"input_tokens":50,"output_tokens":25,"cache_read_input_tokens":400,"cache_creation_input_tokens":50}}}`
	s := cacheSession(claude.Name, raw)
	evidence(s).RawSession["child"] = []json.RawMessage{json.RawMessage(raw)}
	require.NoError(t, checkCacheTokenAccounting(s))
	evidence(s).Usage.CacheCreationInputTokens = 50
	require.Error(t, checkCacheTokenAccounting(s))
}

func TestCodexCacheAccountingRequiresVersion(t *testing.T) {
	s := cacheSession(codex.Name, codexCacheRecord)
	evidence(s).SchemaVersion = "v1"
	require.ErrorContains(t, checkCacheTokenAccounting(s), "schema_version v2")
}

func TestClaudeCacheComparisonDoesNotDeduplicate(t *testing.T) {
	// This check verifies preservation of source counts, not their uniqueness.
	// A source with repeated usage and a matching aggregate still passes.
	raw := `{"type":"assistant","message":{"id":"same-message","usage":{"input_tokens":50,"output_tokens":25,"cache_read_input_tokens":400,"cache_creation_input_tokens":50}}}`
	s := cacheSession(claude.Name, raw, raw)
	require.NoError(t, checkCacheTokenAccounting(s))
}

func TestCacheAccountingNotCovered(t *testing.T) {
	for _, s := range []*Session{
		cacheSession("cursor"),
		cacheSession(codex.Name, `{"type":"token_usage_record","payload":{"response_id":"a","usage":{"input_tokens":1000,"output_tokens":50}}}`),
	} {
		require.ErrorIs(t, checkCacheTokenAccounting(s), errNotCovered)
	}
}

func TestCacheAccountingCompactedSession(t *testing.T) {
	raw, err := os.ReadFile("../codex/testdata/compacted-session.jsonl")
	require.NoError(t, err)
	s := cacheSession(codex.Name)
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		evidence(s).RawSession[tokenMainTranscript] = append(evidence(s).RawSession[tokenMainTranscript], json.RawMessage(line))
	}
	evidence(s).Usage = &aicodingsession.Usage{
		InputTokens: 20425, OutputTokens: 168426, TotalTokens: 188851,
		CacheReadInputTokens: 51113536, CacheCreationInputTokens: 669175, EstimatedCostUSD: 8.5094,
	}
	require.NoError(t, checkCacheTokenAccounting(s))
	// Even normalized, self-consistent versions of the original inflated
	// aggregates must fail the source comparison.
	u := evidence(s).Usage
	u.InputTokens = 5242914636 - 5160057426 - 80308007
	u.OutputTokens = 17901845
	u.TotalTokens = u.InputTokens + u.OutputTokens
	u.CacheReadInputTokens = 5160057426
	u.CacheCreationInputTokens = 80308007
	require.Error(t, checkCacheTokenAccounting(s))
}

func TestCacheAccountingCumulativeSources(t *testing.T) {
	native := `{"type":"token_usage_record","payload":{"thread_token_usage":{"input_tokens":1000,"cached_input_tokens":800,"cache_write_input_tokens":100,"output_tokens":50}}}`
	legacy := `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":800,"cache_write_input_tokens":100,"output_tokens":50}}}}`
	divergent := `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":2000,"cached_input_tokens":1600,"cache_write_input_tokens":200,"output_tokens":100}}}}`
	for _, records := range [][]string{{legacy}, {native, divergent}, {divergent, native, divergent}} {
		require.NoError(t, checkCacheTokenAccounting(cacheSession(codex.Name, records...)))
	}
	// A decrease at the end must retain the earlier accepted counters for
	// each fallback source, rather than comparing with the final raw snapshot.
	smallerNative := `{"type":"token_usage_record","payload":{"thread_token_usage":{"input_tokens":500,"cached_input_tokens":400,"cache_write_input_tokens":50,"output_tokens":25}}}`
	smallerLegacy := `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":500,"cached_input_tokens":400,"cache_write_input_tokens":50,"output_tokens":25}}}}`
	for _, records := range [][]string{{native, smallerNative}, {legacy, smallerLegacy}} {
		require.NoError(t, checkCacheTokenAccounting(cacheSession(codex.Name, records...)))
	}
}
