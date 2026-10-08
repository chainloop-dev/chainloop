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

package action

import (
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
)

// replaceSpecCopies replaces each exact copy of a spec source in the
// transcript of the session with a pointer to its material, when the agent of
// the session can find the copies, and reports the result per finder in the
// debug log. It never fails: a copy that does not match stays inline.
func replaceSpecCopies(provider trace.Provider, evidence *aicodingsession.Evidence, sources pointer.Sources, sessionID string, log zerolog.Logger) {
	replacer, ok := provider.(trace.SpecCopyReplacer)
	if !ok || len(sources) == 0 || len(evidence.Data.RawSession) == 0 {
		return
	}

	for finder, r := range replacer.ReplaceSpecCopies(evidence.Data.RawSession, sources) {
		ev := log.Debug().Str("session", sessionID).Str("finder", finder).
			Int("replaced", r.Replaced).Int("bytes_removed", r.BytesRemoved)
		for reason, n := range r.Skipped {
			ev = ev.Int("not_replaced: "+reason, n)
		}
		ev.Msg("spec source copies in the transcript")
	}
}
