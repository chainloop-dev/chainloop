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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/attribution"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
)

// HandleCommitMsgHook appends a Chainloop-Trace-Sessions trailer to the commit
// message when AI sessions have modified files staged for commit.
// Errors are logged but never returned (to avoid blocking commits).
func HandleCommitMsgHook(_ context.Context, msgFilePath string, log zerolog.Logger) error {
	if err := handleCommitMsg(msgFilePath, log); err != nil {
		log.Debug().Err(err).Msg("commit-msg hook failed")
	}

	return nil
}

func handleCommitMsg(msgFilePath string, log zerolog.Logger) error {
	gitDir, repoRoot, err := tracegit.FindGitDirAndRoot()
	if err != nil {
		return err
	}

	log.Debug().Str("git_dir", gitDir).Msg("commit-msg hook invoked")

	// Skip merge commits — their content is already attributed in the
	// merged-in branch's commits.
	if tracegit.IsMergeInProgress(gitDir) {
		log.Debug().Msg("merge in progress, skipping trailer")

		return nil
	}

	attrs, err := state.NewGitStore(gitDir).LoadAllAILineAttributions()
	if err != nil {
		return fmt.Errorf("load AI line attributions: %w", err)
	}
	if len(attrs) == 0 {
		return nil
	}

	client := tracegit.NewGoGitClient()
	stagedFiles, err := client.StagedFiles(repoRoot)
	if err != nil {
		return fmt.Errorf("get staged files: %w", err)
	}

	sessionIDs := matchSessionsToFiles(attrs, stagedFiles)
	if len(sessionIDs) == 0 {
		return nil
	}

	log.Debug().Strs("session_ids", sessionIDs).Msg("appending trace sessions trailer")

	return appendTrailer(msgFilePath, sessionIDs)
}

// matchSessionsToFiles returns session IDs that have AI line attribution for
// any of the given files. The returned slice is sorted.
func matchSessionsToFiles(attrs []*state.AILineAttribution, files []string) []string {
	fileSet := make(map[string]struct{}, len(files))
	for _, f := range files {
		fileSet[f] = struct{}{}
	}

	var sessionIDs []string
	for _, attr := range attrs {
		for filePath := range attr.Files {
			if _, ok := fileSet[filePath]; ok {
				sessionIDs = append(sessionIDs, attr.SessionID)

				break
			}
		}
	}

	sort.Strings(sessionIDs)

	return sessionIDs
}

// appendTrailer reads the commit message file, appends a Chainloop-Trace-Sessions
// trailer with the given session IDs, and writes the file back.
// Skips if the trailer already exists (idempotent for --amend).
func appendTrailer(msgFilePath string, sessionIDs []string) error {
	content, err := os.ReadFile(msgFilePath)
	if err != nil {
		return fmt.Errorf("read commit message: %w", err)
	}

	msg := string(content)

	if strings.Contains(msg, state.TrailerKey+":") {
		return nil
	}

	trailer := fmt.Sprintf("%s: %s\n", state.TrailerKey, strings.Join(sessionIDs, ", "))

	// Git trailer convention: trailers follow a blank line after the body.
	// Ensure we have at least one blank line before the trailer.
	trimmed := strings.TrimRight(msg, "\n")
	result := trimmed + "\n\n" + trailer

	return os.WriteFile(msgFilePath, []byte(result), 0600)
}

// HandlePostCommitHook handles the post-commit git hook.
// Errors are logged but never returned (to avoid blocking commits).
func HandlePostCommitHook(ctx context.Context, log zerolog.Logger) error {
	if err := handlePostCommit(ctx, log); err != nil {
		log.Debug().Err(err).Msg("post-commit hook failed")
	}

	return nil
}

func handlePostCommit(_ context.Context, log zerolog.Logger) error {
	gitDir, repoRoot, err := tracegit.FindGitDirAndRoot()
	if err != nil {
		return err
	}

	log.Debug().Str("git_dir", gitDir).Msg("post-commit hook invoked")

	client := tracegit.NewGoGitClient()
	sha, message, err := client.CommitHeadInfo(repoRoot)
	if err != nil {
		return fmt.Errorf("get commit info: %w", err)
	}

	// Skip merge commits: their content is already attributed in the
	// merged-in branch's commits, so recording them would double-count.
	isMerge, err := client.IsMergeCommit(repoRoot, sha)
	if err != nil {
		return fmt.Errorf("check merge commit: %w", err)
	}
	if isMerge {
		log.Debug().Str("sha", sha).Msg("merge commit, skipping record")

		return nil
	}

	// Prefer the trailer's session IDs when present — they survive rebase
	// and cherry-pick verbatim. Fall back to file-based derivation otherwise.
	store := state.NewGitStore(gitDir)

	sessionIDs := state.ParseSessionIDsFromTrailer(message)
	if len(sessionIDs) == 0 {
		sessionIDs = deriveSessionIDsForCommit(client, store, repoRoot, sha)
	}

	log.Debug().Str("sha", sha).Strs("session_ids", sessionIDs).Msg("recording commit")

	rec := &state.CommitRecord{
		SHA:        sha,
		Message:    message,
		SessionIDs: sessionIDs,
		Timestamp:  state.NowTimestamp(),
	}

	if err := store.SaveCommitRecord(rec); err != nil {
		return fmt.Errorf("save commit record: %w", err)
	}

	log.Info().Str("sha", sha).Msg("commit record saved")

	return nil
}

// deriveSessionIDsForCommit scans ai-lines data and returns session IDs
// that have modified any file that appears in the given commit.
func deriveSessionIDsForCommit(client tracegit.Client, store *state.Store, repoRoot, headSHA string) []string {
	attrs, err := store.LoadAllAILineAttributions()
	if err != nil || len(attrs) == 0 {
		return nil
	}

	changes, err := client.CodeChangesForRange(repoRoot, headSHA, headSHA)
	if err != nil || len(changes.Files) == 0 {
		return nil
	}

	files := make([]string, 0, len(changes.Files))
	for _, f := range changes.Files {
		files = append(files, f.Path)
	}

	return matchSessionsToFiles(attrs, files)
}

// filterCurrentBranchCommits returns the subset of records whose SHA is
// reachable from HEAD but not from the default remote branch (i.e. on-branch),
// and which are not merge commits. Records pointing at SHAs that no longer
// exist in the branch — typically left behind by a rebase — are dropped.
//
// If the branch SHA list cannot be obtained, returns the input unchanged
// (best-effort: missing context shouldn't block attestation).
func filterCurrentBranchCommits(client tracegit.Client, repoRoot string, records []*state.CommitRecord, log zerolog.Logger) []*state.CommitRecord {
	branchSHAs, err := client.BranchCommitSHAs(repoRoot)
	if err != nil {
		log.Debug().Err(err).Msg("could not enumerate branch commits, skipping orphan filter")

		return records
	}

	inBranch := make(map[string]struct{}, len(branchSHAs))
	for _, sha := range branchSHAs {
		inBranch[sha] = struct{}{}
	}

	filtered := make([]*state.CommitRecord, 0, len(records))
	for _, rec := range records {
		if _, ok := inBranch[rec.SHA]; !ok {
			log.Debug().Str("sha", rec.SHA).Msg("dropping orphan commit record (not in branch)")
			continue
		}

		isMerge, err := client.IsMergeCommit(repoRoot, rec.SHA)
		if err != nil {
			log.Debug().Err(err).Str("sha", rec.SHA).Msg("could not check merge status, dropping defensively")
			continue
		}
		if isMerge {
			log.Debug().Str("sha", rec.SHA).Msg("dropping merge commit record")
			continue
		}

		filtered = append(filtered, rec)
	}

	return filtered
}

// RunTracePushOpts configures RunTracePush behaviour.
type RunTracePushOpts struct {
	// AllowEmpty makes the push attest every recorded session even when
	// no AI-attributed commits exist. Used by `chainloop trace run`,
	// which drives one-shot agent sessions that may produce no commits.
	AllowEmpty bool

	// ProjectName, when set, overrides the projectName read from
	// .chainloop.yml. Lets `trace run` operate without mutating the repo
	// config.
	ProjectName string
	// Organization, when set, overrides the organization read from
	// .chainloop.yml.
	Organization string
	// WorkflowName, when set, overrides the workflowName read from
	// .chainloop.yml.
	WorkflowName string
	// ProjectVersion, when set, targets a specific project version for
	// the attestation. Empty means use the latest version.
	ProjectVersion string
	// IgnoreYAML disables the .chainloop.yml fallback used to fill in
	// identity fields. `trace run` sets this so its attestations depend
	// only on CLI flags. Pre-push hook callers leave it false to keep
	// reading the repo config.
	IgnoreYAML bool

	// ActionOpts is the root command's initialized options, used to build
	// the attestation executor. Required: the push cannot run without it.
	ActionOpts *ActionsOpts
	// CLIVersion is the bare CLI version recorded in the attestation
	// predicate.
	CLIVersion string
}

// HandlePrePushHook handles the pre-push git hook.
// When requireTrace is true, errors from the attestation push are
// propagated so that the git push is blocked. When false, errors are
// logged but never returned.
func HandlePrePushHook(ctx context.Context, requireTrace bool, log zerolog.Logger, opts RunTracePushOpts) error {
	drainPushStdin()

	if err := RunTracePush(ctx, log, opts); err != nil {
		if requireTrace {
			return fmt.Errorf("attestation failed (--require-trace is enabled): %w", err)
		}

		log.Debug().Err(err).Msg("pre-push hook failed")
	}

	return nil
}

// RunTracePush gathers AI-coding-session evidence from the local trace
// state and emits a Chainloop attestation. Shared between the pre-push
// git hook and `chainloop trace run`.
//
//nolint:gocyclo // straight-line orchestration of the pre-push flow; splitting it would only move the branches
func RunTracePush(ctx context.Context, log zerolog.Logger, opts RunTracePushOpts) error {
	store, repoRoot, err := state.Locate()
	if err != nil {
		return err
	}

	log.Debug().Str("state_dir", store.Dir()).Str("repo_root", repoRoot).Bool("allow_empty", opts.AllowEmpty).Msg("trace push invoked")

	allCommits, err := store.LoadAllCommitRecords()
	if err != nil {
		return fmt.Errorf("load commit records: %w", err)
	}
	log.Debug().Int("commit_count", len(allCommits)).Msg("loaded commit records")

	gitClient := tracegit.NewGoGitClient()

	// Drop orphan records (SHAs no longer in the branch — typically left over
	// by a rebase) and any record that turns out to belong to a merge commit.
	allCommits = filterCurrentBranchCommits(gitClient, repoRoot, allCommits, log)

	// Discard commits without any session IDs (no AI agent involved)
	var aiCommits []*state.CommitRecord
	var untrackedAICommits int
	for _, c := range allCommits {
		if len(c.SessionIDs) > 0 {
			aiCommits = append(aiCommits, c)
			if !c.Tracked {
				untrackedAICommits++
			}
		}
	}

	if len(aiCommits) == 0 && !opts.AllowEmpty {
		log.Info().Msg("no AI-assisted commits, skipping attestation")

		return nil
	}

	// If every candidate AI commit has already been attested by a previous
	// successful push, there is nothing new to report. Short-circuit to avoid
	// emitting a duplicate attestation on no-op `git push` invocations
	// (including `git tag && git push --tags` over already-pushed commits).
	if len(aiCommits) > 0 && untrackedAICommits == 0 && !opts.AllowEmpty {
		log.Info().Int("ai_commit_count", len(aiCommits)).Msg("all AI-assisted commits already attested, skipping push")

		return nil
	}

	sessionCommits := make(map[string][]*state.CommitRecord)
	for _, c := range aiCommits {
		for _, sid := range c.SessionIDs {
			sessionCommits[sid] = append(sessionCommits[sid], c)
		}
	}
	for _, commits := range sessionCommits {
		sort.Slice(commits, func(i, j int) bool {
			return commits[i].Timestamp < commits[j].Timestamp
		})
	}

	sessionRecords, err := store.LoadAllSessionRecords()
	if err != nil {
		log.Debug().Err(err).Msg("could not load session records")
	}

	if len(aiCommits) == 0 {
		for sid := range sessionRecords {
			sessionCommits[sid] = nil
		}
		if len(sessionCommits) == 0 {
			log.Info().Msg("no AI coding sessions recorded, skipping attestation")

			return nil
		}
		log.Info().Int("session_count", len(sessionCommits)).Msg("no AI-assisted commits; attesting session evidence only")
	} else {
		log.Info().Int("ai_commit_count", len(aiCommits)).Msg("parsing AI-assisted commit records")
	}

	log.Debug().Int("session_count", len(sessionCommits)).Msg("grouped commits by session")

	// Collect repo-level context once (same for all sessions)
	rawDir := store.RawSessionDir()

	gitCtx, gitCtxErr := gitClient.Context(repoRoot)
	if gitCtxErr != nil {
		log.Debug().Err(gitCtxErr).Msg("could not collect git context")
	}

	isGenerated := gitClient.GeneratedMatcher(repoRoot)

	// Build evidence for each session
	type sessionEvidence struct {
		sessionID string
		tmpPath   string
	}
	var evidenceFiles []sessionEvidence

	// Register cleanup before the loop so mid-loop returns are covered
	defer func() {
		for _, ef := range evidenceFiles {
			_ = os.Remove(ef.tmpPath)
		}
	}()

	for sessionID, commits := range sessionCommits {
		log.Debug().Str("session", sessionID).Int("commits", len(commits)).Msg("processing session")

		provider := providerForSession(sessionRecords, sessionID)
		if provider == nil {
			log.Debug().Str("session", sessionID).Msg("no provider registered for session, skipping")
			continue
		}

		// Fresh copy of session data before parsing — the session-start copy
		// may be stale if more conversation happened between start and push.
		if err := provider.CopySessionData(store, repoRoot, sessionID); err != nil {
			log.Debug().Err(err).Str("session", sessionID).Msg("could not refresh session data")
		}

		parseOpts := &trace.ParseOpts{
			SessionDir: rawDir,
			SessionID:  sessionID,
		}
		if rec, ok := sessionRecords[sessionID]; ok && rec != nil {
			parseOpts.AgentVersion = rec.AgentVersion
			parseOpts.Model = rec.Model
		}

		result, err := provider.ParseSession(ctx, parseOpts)
		if err != nil {
			log.Debug().Err(err).Str("session", sessionID).Msg("could not parse session, skipping")
			continue
		}

		// Apply repo-wide context with per-session commit overrides
		if gitCtxErr == nil {
			sessionCtx := *gitCtx
			sessionCtx.Commits = commitDescriptions(commits)
			sessionCtx.CommitCount = len(commits)
			if len(commits) > 0 {
				sessionCtx.CommitStart = commits[0].SHA
				sessionCtx.CommitEnd = commits[len(commits)-1].SHA
			}
			result.Data.GitContext = &sessionCtx
		}

		// Collect code changes scoped to this session's commits. Use a SET-based
		// API rather than a SHA range: when the session's commits are non-
		// contiguous on the branch (e.g. interleaved with another session or
		// human commits), a range diff would over-count by including the gap.
		if len(commits) > 0 {
			shas := make([]string, 0, len(commits))
			for _, c := range commits {
				shas = append(shas, c.SHA)
			}
			codeChanges, err := gitClient.CodeChangesForCommits(repoRoot, shas)
			if err != nil {
				log.Debug().Err(err).Msg("could not collect code changes")
			} else {
				attr := store.LoadAILineAttribution(sessionID)
				attribution.FilterGenerated(codeChanges, isGenerated)
				attribution.Enrich(sessionID, attr.Files, codeChanges)
				result.Data.CodeChanges = codeChanges
			}
		}

		// Write evidence to temp file
		tmpFile, err := os.CreateTemp("", "chainloop-trace-*.json")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}

		enc := json.NewEncoder(tmpFile)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())

			return fmt.Errorf("write evidence: %w", err)
		}
		_ = tmpFile.Close()

		log.Debug().Str("session", sessionID).Msg("generated evidence")

		evidenceFiles = append(evidenceFiles, sessionEvidence{
			sessionID: sessionID,
			tmpPath:   tmpFile.Name(),
		})
	}

	if len(evidenceFiles) == 0 {
		log.Debug().Msg("no session evidence could be generated")

		return nil
	}

	// Create attestation using local state file to avoid conflicts with other attestations
	localStatePath := store.AttestationStatePath()
	defer func() { _ = os.Remove(localStatePath) }()

	projectName, organization, workflowName := resolvePushIdentity(repoRoot, opts)
	if projectName == "" {
		if opts.IgnoreYAML {
			return fmt.Errorf("no project name provided")
		}

		return fmt.Errorf("no project name found (pass via options or .chainloop.yml)")
	}

	log.Debug().Str("project", projectName).Msg("resolved project for trace push")

	execOpts := []ExecutorOption{WithLocalStatePath(localStatePath), WithLogger(log)}
	if organization != "" {
		log.Debug().Str("forced_org", organization).Msg("forcing organization for trace push")
		execOpts = append(execOpts, WithForcedOrganization(organization))
	}
	executor, err := NewAttestationExecutor(opts.ActionOpts, opts.CLIVersion, execOpts...)
	if err != nil {
		// Warn (not debug): a misconfiguration here means no attestation is
		// sent at all, and the pre-push hook swallows the returned error
		// unless require-trace is on.
		log.Warn().Err(err).Msg("skipping trace attestation")

		return err
	}
	defer func() { _ = executor.Close() }()

	attestationID, err := executor.Init(ctx, workflowName, projectName, opts.ProjectVersion)
	if err != nil {
		return fmt.Errorf("attestation init: %w", err)
	}

	log.Debug().Str("attestation_id", attestationID).Msg("attestation initialized")

	// Add evidence for each session
	var addedCount int
	for _, ef := range evidenceFiles {
		name := evidenceName(ef.sessionID)
		if err := executor.AddEvidence(ctx, name, ef.tmpPath); err != nil {
			log.Debug().Err(err).Str("session", ef.sessionID).Msg("could not add evidence")
			continue
		}
		addedCount++
		log.Debug().Str("session", ef.sessionID).Str("name", name).Msg("evidence added")
	}

	if addedCount == 0 {
		log.Debug().Msg("no evidence successfully added, resetting attestation")
		_ = executor.Reset(ctx, "trace-push", "no CHAINLOOP_AI_CODING_SESSION evidence added")

		return nil
	}

	// Push attestation
	log.Debug().Msg("pushing attestation")
	if err := executor.Push(ctx); err != nil {
		return fmt.Errorf("attestation push: %w", err)
	}

	log.Debug().Msg("attestation pushed, wiping single-use trace state")

	// Mark every AI commit included in this attestation as tracked so that a
	// subsequent `git push` with no new commits short-circuits at the skip
	// check above. Save errors are non-fatal: the attestation already went
	// out, and at worst we re-attest the same commits on the next push.
	for _, c := range aiCommits {
		if c.Tracked {
			continue
		}
		c.Tracked = true
		if err := store.SaveCommitRecord(c); err != nil {
			log.Debug().Err(err).Str("sha", c.SHA).Msg("could not mark commit record as tracked")
		}
	}

	_ = store.WipeTraceDir()
	if liveSHAs, err := gitClient.LocalReachableSHAs(repoRoot); err == nil {
		if err := store.GCOrphans(liveSHAs); err != nil {
			log.Debug().Err(err).Msg("orphan GC failed; trace state is intact but not pruned")
		}
	} else {
		log.Debug().Err(err).Msg("could not enumerate local branch SHAs; skipping orphan GC")
	}

	return nil
}

// evidenceName returns the material name for a session evidence document.
// Format: ai-coding-session-<first 6 chars of session ID>. Underscores are
// replaced with hyphens because material names may only contain lowercase
// letters, numbers, and hyphens.
func evidenceName(sessionID string) string {
	short := sessionID
	if len(short) > 6 {
		short = short[:6]
	}

	return "ai-coding-session-" + strings.ReplaceAll(short, "_", "-")
}

// commitDescriptions returns commit strings in "SHA first_line_of_message" format.
func commitDescriptions(commits []*state.CommitRecord) []string {
	descs := make([]string, len(commits))
	for i, c := range commits {
		firstLine, _, _ := strings.Cut(c.Message, "\n")
		firstLine = strings.TrimSpace(firstLine)
		if firstLine != "" {
			descs[i] = c.SHA + " " + firstLine
		} else {
			descs[i] = c.SHA
		}
	}

	return descs
}

// resolvePushIdentity returns the project name, organization, and
// workflow name to use for an attestation push. Opts override
// .chainloop.yml; YAML fields fill in whatever opts left empty unless
// opts.IgnoreYAML is set, in which case the YAML is skipped entirely.
// Workflow always passes through ResolveWorkflowName so the default kicks in.
func resolvePushIdentity(repoRoot string, opts RunTracePushOpts) (project, org, workflow string) {
	project = opts.ProjectName
	org = opts.Organization
	workflow = opts.WorkflowName

	if !opts.IgnoreYAML && (project == "" || org == "" || workflow == "") {
		if yml := config.FindChainloopYML(repoRoot); yml != nil {
			if project == "" {
				project = yml.ProjectName
			}
			if org == "" {
				org = yml.Organization
			}
			if workflow == "" {
				workflow = yml.WorkflowName
			}
		}
	}

	return project, org, config.ResolveWorkflowName(workflow)
}

// drainPushStdin reads and discards pre-push stdin to avoid SIGPIPE.
func drainPushStdin() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
	}
}

// providerForSession resolves the trace.Provider that owns sessionID.
// Session records written before multi-provider support had no provider
// field and are attributed to trace.DefaultProviderName.
func providerForSession(records map[string]*state.SessionRecord, sessionID string) trace.Provider {
	name := trace.DefaultProviderName

	if rec, ok := records[sessionID]; ok && rec != nil && rec.Provider != "" {
		return providers.ByName(rec.Provider)
	}

	return providers.ByName(name)
}
