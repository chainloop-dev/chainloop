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
	"crypto/sha256"
	"errors"
	"fmt"
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/attribution"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
)

func tagShellCommand(provider trace.Provider, input *trace.HookInput, log zerolog.Logger) {
	tagger, ok := provider.(trace.ShellCommandTagger)
	if !ok {
		return
	}

	token, err := state.EncodeShellCallKey(shellCallKey(input))
	if err != nil {
		log.Debug().Err(err).Msg("pre-command: encode shell call key failed")
		return
	}
	if err := tagger.TagShellCommand(input, token); err != nil {
		log.Debug().Err(err).Msg("pre-command: tag shell command failed")
	}
}

// checkpointActiveShellAttribution records index changes made by the exact
// shell call whose token git inherited. The normal PostToolUse attribution
// arrives only after the call exits, after commit-msg and pre-push already ran.
func checkpointActiveShellAttribution(store *state.Store, repoRoot string, stagedFiles []string) error {
	if len(stagedFiles) == 0 {
		return nil
	}

	token := os.Getenv(state.ShellCallEnv)
	if token == "" {
		return nil
	}
	key, err := state.DecodeShellCallKey(token)
	if err != nil {
		return err
	}

	beforeWorktree, err := store.LoadShellPreSignature(key)
	if err != nil {
		return fmt.Errorf("load pre-command worktree snapshot: %w", err)
	}
	beforeIndex, err := store.LoadShellPreIndexSignature(key)
	if err != nil {
		return fmt.Errorf("load pre-command index snapshot: %w", err)
	}
	beforeHeads, err := store.LoadShellPreHeads(key)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load pre-command HEAD commits: %w", err)
	}

	client := tracegit.NewGitClient()
	afterIndex, err := client.SnapshotIndex(repoRoot)
	if err != nil {
		return fmt.Errorf("snapshot staged index: %w", err)
	}
	staged := make(map[string]struct{}, len(stagedFiles))
	for _, path := range stagedFiles {
		staged[path] = struct{}{}
	}

	var errs []error
	open := store.LoadAILineAttribution(key.SessionID).Open
	for root, indexBefore := range beforeIndex {
		if !sameDir(root, repoRoot) {
			continue
		}
		worktreeBefore, ok := beforeWorktree[root]
		if !ok {
			continue
		}

		changed, deleted := attribution.ChangedPaths(indexBefore, afterIndex)
		stagedPaths := make([]string, 0, len(changed)+len(deleted))
		stagedHashes := make(map[string]string)
		for _, relPath := range append(changed, deleted...) {
			if _, ok := staged[relPath]; !ok {
				continue
			}
			stagedPaths = append(stagedPaths, relPath)
			if hash, exists := afterIndex[relPath]; exists {
				stagedHashes[relPath] = hash
			}
		}
		stagedContent, err := client.IndexBlobs(repoRoot, stagedHashes)
		if err != nil {
			return fmt.Errorf("read staged blobs: %w", err)
		}
		base := beforeHeads[root]
		if base == "" {
			base = "HEAD"
		}
		beforeContents, _ := client.FilesAt(repoRoot, base, stagedPaths)
		checkpointed := false
		for _, relPath := range stagedPaths {
			beforeContent := beforeContents[relPath]
			var afterContent []byte
			if hash, exists := afterIndex[relPath]; exists {
				afterContent = stagedContent[relPath]
				indexBefore[relPath] = hash
				// SnapshotWorktree also hashes raw bytes with SHA-256.
				// Checkpointing to the staged blob makes PostToolUse retain
				// any different or missing worktree content, including when
				// the call restored the exact bytes from before it started.
				worktreeBefore[relPath] = fmt.Sprintf("%x", sha256.Sum256(afterContent))
			} else {
				delete(indexBefore, relPath)
				// The staged content is absence. Checkpoint the worktree to the
				// same state so PostToolUse records any recreated path, even
				// when its bytes equal the file from before this call.
				delete(worktreeBefore, relPath)
			}

			edit := attribution.DropOpen(attribution.ComputeEdit(beforeContent, afterContent), open[relPath])
			if edit.Changed {
				if err := store.RecordEdit(key.SessionID, relPath, editRecord(edit)); err != nil {
					errs = append(errs, fmt.Errorf("record attribution for %s: %w", relPath, err))
					continue
				}
			}
			checkpointed = true
		}

		if checkpointed {
			if err := store.SaveShellPreSignature(key, beforeWorktree); err != nil {
				errs = append(errs, fmt.Errorf("checkpoint worktree snapshot: %w", err))
			}
			if err := store.SaveShellPreIndexSignature(key, beforeIndex); err != nil {
				errs = append(errs, fmt.Errorf("checkpoint index snapshot: %w", err))
			}
		}
	}

	return errors.Join(errs...)
}

func advanceActiveShellHead(store *state.Store, repoRoot, sha string) error {
	token := os.Getenv(state.ShellCallEnv)
	if token == "" {
		return nil
	}
	key, err := state.DecodeShellCallKey(token)
	if err != nil {
		return err
	}

	heads, err := store.LoadShellPreHeads(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if heads == nil {
		heads = make(map[string]string)
	}

	updated := false
	for root := range heads {
		if sameDir(root, repoRoot) {
			heads[root] = sha
			updated = true
		}
	}
	if !updated {
		heads[repoRoot] = sha
	}

	return store.SaveShellPreHeads(key, heads)
}
