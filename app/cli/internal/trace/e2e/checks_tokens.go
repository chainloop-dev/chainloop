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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/codex"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pi"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// These checks compare the evidence stored at push with its source records.
// A self-consistent aggregate alone would not catch the Codex overcount: both
// inflated input and total could still satisfy total = input + output.
func checkCacheTokenAccounting(s *Session) error {
	d := evidence(s)
	var want aicodingsession.Usage
	switch d.Agent.Name {
	case codex.Name:
		if d.SchemaVersion != "v2" {
			return errors.New("Codex cache accounting requires evidence schema_version v2")
		}
		var err error
		want, err = codexSourceUsage(d.RawSession["main"])
		if err != nil {
			return err
		}
	case pi.Name:
		// Pi raw_session contains only the selected branch. Read native fields
		// and child aggregate usage independently of the provider parser.
		want = piSourceUsage(d.RawSession["main"])
		want.EstimatedCostUSD = 0
		if want.OutputTokens > int(^uint(0)>>1)-want.InputTokens {
			return errors.New("Pi source input + output token total overflows")
		}
	case claude.Name:
		for _, records := range d.RawSession {
			for _, raw := range records {
				var r struct {
					Type    string `json:"type"`
					Message struct {
						Usage aicodingsession.Usage `json:"usage"`
					} `json:"message"`
				}
				if json.Unmarshal(raw, &r) == nil && r.Type == "assistant" {
					want.InputTokens += r.Message.Usage.InputTokens
					want.OutputTokens += r.Message.Usage.OutputTokens
					want.CacheReadInputTokens += r.Message.Usage.CacheReadInputTokens
					want.CacheCreationInputTokens += r.Message.Usage.CacheCreationInputTokens
				}
			}
		}
	default:
		return fmt.Errorf("%w: cache accounting source comparison is not implemented for %s", errNotCovered, d.Agent.Name)
	}
	if want.CacheReadInputTokens+want.CacheCreationInputTokens == 0 {
		return fmt.Errorf("%w: this session did not exercise cache reads or writes", errNotCovered)
	}
	want.TotalTokens = want.InputTokens + want.OutputTokens
	if err := checkTokenUsage(s); err != nil {
		return err
	}
	got := *d.Usage
	got.EstimatedCostUSD = 0 // Prices vary by model; provider regressions check the rates.
	if got != want {
		return fmt.Errorf("stored token counts do not match source usage with cache separated: got %+v, want %+v", got, want)
	}
	return nil
}

// sourceTokens keeps Codex's inclusive input until the source totals are known.
type sourceTokens struct {
	Input  int `json:"input_tokens"`
	Read   int `json:"cached_input_tokens"`
	Write  int `json:"cache_write_input_tokens"`
	Output int `json:"output_tokens"`
	Total  int `json:"total_tokens"`
}

// Cumulative fallback keeps the last accepted high-water snapshot, rather than
// the final raw record. A stale record at the end must not change the expected
// evidence. Validate the cache partition of the increment too: a correction
// that moves old base input into cache cannot be treated as new usage.
func acceptSourceSnapshot(previous, next sourceTokens) sourceTokens {
	if next.Input < previous.Input || next.Output < previous.Output ||
		next.Read < previous.Read || next.Write < previous.Write || next.Total < previous.Total {
		return previous
	}
	input := next.Input - previous.Input
	read := next.Read - previous.Read
	write := next.Write - previous.Write
	if read > input || write > input-read {
		return previous
	}
	return next
}

func codexSourceUsage(records []json.RawMessage) (aicodingsession.Usage, error) {
	var responses, nativeTotal, legacyTotal sourceTokens
	haveNative, completeResponses, haveLegacy := false, true, false
	seen := make(map[string]bool)
	for _, raw := range records {
		var r struct {
			Type    string `json:"type"`
			Payload struct {
				Type       string        `json:"type"`
				ResponseID string        `json:"response_id"`
				Usage      *sourceTokens `json:"usage"`
				Thread     *sourceTokens `json:"thread_token_usage"`
				Info       struct {
					Total *sourceTokens `json:"total_token_usage"`
				} `json:"info"`
			} `json:"payload"`
		}
		if json.Unmarshal(raw, &r) != nil {
			continue
		}
		p := r.Payload
		switch r.Type {
		case "token_usage_record":
			if p.Usage == nil && p.Thread == nil {
				continue
			}
			haveNative = true
			if p.Thread != nil {
				nativeTotal = acceptSourceSnapshot(nativeTotal, *p.Thread)
			}
			if p.ResponseID == "" || p.Usage == nil {
				completeResponses = false
				continue
			}
			if !seen[p.ResponseID] {
				responses.Input += p.Usage.Input
				responses.Read += p.Usage.Read
				responses.Write += p.Usage.Write
				responses.Output += p.Usage.Output
				seen[p.ResponseID] = true
			}
		case "event_msg":
			if p.Type == "token_count" && p.Info.Total != nil {
				legacyTotal = acceptSourceSnapshot(legacyTotal, *p.Info.Total)
				haveLegacy = true
			}
		}
	}
	var total sourceTokens
	switch {
	case haveNative && completeResponses:
		total = responses
	case haveNative:
		total = nativeTotal
	case haveLegacy:
		total = legacyTotal
	default:
		return aicodingsession.Usage{}, errors.New("the Codex transcript has no source token usage")
	}
	return aicodingsession.Usage{
		InputTokens:              total.Input - total.Read - total.Write,
		OutputTokens:             total.Output,
		CacheReadInputTokens:     total.Read,
		CacheCreationInputTokens: total.Write,
	}, nil
}
