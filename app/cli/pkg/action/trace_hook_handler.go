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
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/repositoryconfig"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/attribution"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
)

// HandleCommitMsgHook appends a Chainloop-Trace-Sessions trailer to the commit
// message when AI sessions have modified files staged for commit, or when the
// commit amends one that has the trailer.
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

	client := tracegit.NewGoGitClient()

	// An amend with a new message replaces HEAD's trailer, and its sessions
	// cannot be matched again because HEAD already consumed their pending
	// ranges. The amended commit still holds HEAD's lines, so HEAD's trailer
	// is kept.
	var sessionIDs []string
	if isAmendOfHead(client, repoRoot) {
		_, headMsg, err := client.CommitHeadInfo(repoRoot)
		if err != nil {
			return fmt.Errorf("get HEAD commit: %w", err)
		}

		sessionIDs = state.ParseSessionIDsFromTrailer(headMsg)
		log.Debug().Strs("session_ids", sessionIDs).Msg("amending HEAD, keeping its trace sessions")
	}

	attrs, err := state.NewGitStore(gitDir).LoadAllAILineAttributions()
	if err != nil {
		return fmt.Errorf("load AI line attributions: %w", err)
	}
	if len(attrs) > 0 {
		stagedFiles, err := client.StagedFiles(repoRoot)
		if err != nil {
			return fmt.Errorf("get staged files: %w", err)
		}

		sessionIDs = mergeSessionIDs(sessionIDs, matchSessionsToFiles(attrs, stagedFiles))
	}

	if len(sessionIDs) == 0 {
		return nil
	}

	log.Debug().Strs("session_ids", sessionIDs).Msg("appending trace sessions trailer")

	return appendTrailer(msgFilePath, sessionIDs)
}

// isAmendOfHead reports whether the commit being made replaces HEAD. Git gives
// the commit-msg hook no direct sign of an amend done with -m, but it exports
// the author date of the new commit in GIT_AUTHOR_DATE as "@<unix> <tz>". An
// amend keeps the author date of HEAD, and any other commit, --allow-empty
// included, carries the current time.
//
// An amend with --reset-author, or an author date that the user set in
// another format, is not seen here. The post-rewrite hook still carries the
// sessions over to the local record for those.
func isAmendOfHead(client tracegit.Client, repoRoot string) bool {
	raw, ok := strings.CutPrefix(os.Getenv("GIT_AUTHOR_DATE"), "@")
	if !ok {
		return false
	}

	secs, _, _ := strings.Cut(raw, " ")
	authored, err := strconv.ParseInt(secs, 10, 64)
	if err != nil {
		return false
	}

	headAuthored, err := client.HeadAuthorTime(repoRoot)
	if err != nil {
		return false
	}

	return headAuthored.Unix() == authored
}

// fileOwner is the session currently credited with a staged file's pending
// attribution, and when it made the edit.
type fileOwner struct {
	sessionID string
	editedAt  time.Time
}

// supersededBy reports whether an edit made at editedAt by sessionID takes the
// file over from this owner. Equal timestamps fall to the lower session ID, so
// the trailer stays stable instead of following map iteration order.
func (o fileOwner) supersededBy(sessionID string, editedAt time.Time) bool {
	if editedAt.Equal(o.editedAt) {
		return sessionID < o.sessionID
	}

	return editedAt.After(o.editedAt)
}

// matchSessionsToFiles returns the session IDs that own uncommitted AI line
// attribution for any of the given files. The returned slice is sorted.
//
// Matching is deliberately against Pending rather than Files: a session's full
// range history outlives its commits, so intersecting it would credit a
// long-finished session on every later commit that touches a file it once
// edited, and that commit's whole diff would then be attributed to it.
//
// Each file is owned by the session that edited it last. When two sessions both
// have it pending, the earlier edit no longer describes the file — the later
// session rewrote those lines — so crediting both would attach a session that
// has moved on to this commit and to the pull request carrying it.
func matchSessionsToFiles(attrs []*state.AILineAttribution, files []string) []string {
	fileSet := make(map[string]struct{}, len(files))
	for _, f := range files {
		fileSet[f] = struct{}{}
	}

	owners := make(map[string]fileOwner, len(fileSet))
	for _, attr := range attrs {
		for filePath, edit := range attr.Pending {
			if _, staged := fileSet[filePath]; !staged {
				continue
			}
			if cur, claimed := owners[filePath]; claimed && !cur.supersededBy(attr.SessionID, edit.At) {
				continue
			}

			owners[filePath] = fileOwner{sessionID: attr.SessionID, editedAt: edit.At}
		}
	}

	seen := make(map[string]struct{}, len(owners))
	var sessionIDs []string
	for _, o := range owners {
		if _, dup := seen[o.sessionID]; dup {
			continue
		}
		seen[o.sessionID] = struct{}{}
		sessionIDs = append(sessionIDs, o.sessionID)
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

	// The credited sessions' pending ranges have now landed, so retire them.
	// Skipping this is what makes a session accumulate foreign commits: its
	// files stay pending and keep matching every later commit that touches
	// them. Post-commit errors only ever get logged — the commit already
	// exists — so returning here costs nothing but surfaces the failure.
	if len(sessionIDs) > 0 {
		committed := commitFiles(client, repoRoot, sha)
		for _, sid := range sessionIDs {
			if err := store.MarkConsumed(sid, committed, sha); err != nil {
				return fmt.Errorf("mark attribution consumed for session %s: %w", sid, err)
			}
		}
	}

	log.Info().Str("sha", sha).Msg("commit record saved")

	return nil
}

// HandlePostRewriteHook handles the post-rewrite git hook, which git runs after
// `commit --amend` and `rebase` with one "<old-sha> <new-sha> [extra]" line per
// rewritten commit on stdin.
// Errors are logged but never returned (to avoid blocking the rewrite).
func HandlePostRewriteHook(_ context.Context, log zerolog.Logger) error {
	if err := handlePostRewrite(os.Stdin, log); err != nil {
		log.Debug().Err(err).Msg("post-rewrite hook failed")
	}

	return nil
}

// handlePostRewrite copies the session IDs of each rewritten commit onto the
// record of the commit that replaced it. Without this, an amend that replaces
// the message loses the trailer, and post-commit cannot derive the sessions
// again because the original commit already consumed their pending ranges. The
// old record is then dropped as an orphan at push time, taking the sessions
// with it.
//
// Only records that post-commit already wrote are updated: the rewrite hook
// carries sessions forward, it does not decide which commits are recorded.
func handlePostRewrite(r io.Reader, log zerolog.Logger) error {
	gitDir, _, err := tracegit.FindGitDirAndRoot()
	if err != nil {
		return err
	}

	log.Debug().Str("git_dir", gitDir).Msg("post-rewrite hook invoked")

	store := state.NewGitStore(gitDir)
	records, err := store.LoadAllCommitRecords()
	if err != nil {
		return fmt.Errorf("load commit records: %w", err)
	}

	bySHA := make(map[string]*state.CommitRecord, len(records))
	for _, rec := range records {
		bySHA[rec.SHA] = rec
	}

	// A squash maps several old commits onto one new commit, so collect the
	// sessions of all of them before writing.
	inherited := make(map[string][]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		if old, ok := bySHA[fields[0]]; ok && len(old.SessionIDs) > 0 {
			inherited[fields[1]] = append(inherited[fields[1]], old.SessionIDs...)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read rewritten commits: %w", err)
	}

	for newSHA, sessionIDs := range inherited {
		rec, ok := bySHA[newSHA]
		if !ok {
			log.Debug().Str("sha", newSHA).Msg("no record for rewritten commit, skipping")
			continue
		}

		merged := mergeSessionIDs(rec.SessionIDs, sessionIDs)
		if len(merged) == len(rec.SessionIDs) {
			continue
		}

		log.Debug().Str("sha", newSHA).Strs("session_ids", merged).Msg("carrying sessions over to rewritten commit")

		rec.SessionIDs = merged
		if err := store.SaveCommitRecord(rec); err != nil {
			return fmt.Errorf("save commit record %s: %w", newSHA, err)
		}
	}

	return nil
}

// mergeSessionIDs returns the sorted union of a and b without duplicates.
func mergeSessionIDs(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, id := range list {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}

	sort.Strings(out)

	return out
}

// deriveSessionIDsForCommit scans ai-lines data and returns session IDs
// that have uncommitted attribution for any file in the given commit.
func deriveSessionIDsForCommit(client tracegit.Client, store *state.Store, repoRoot, headSHA string) []string {
	attrs, err := store.LoadAllAILineAttributions()
	if err != nil || len(attrs) == 0 {
		return nil
	}

	files := commitFiles(client, repoRoot, headSHA)
	if len(files) == 0 {
		return nil
	}

	return matchSessionsToFiles(attrs, files)
}

// commitFiles returns the paths the commit at sha changed relative to its
// parent. Returns nil when the diff cannot be read, which leaves callers
// treating the commit as touching nothing rather than failing the hook.
func commitFiles(client tracegit.Client, repoRoot, sha string) []string {
	changes, err := client.CodeChangesForRange(repoRoot, sha, sha)
	if err != nil || len(changes.Files) == 0 {
		return nil
	}

	files := make([]string, 0, len(changes.Files))
	for _, f := range changes.Files {
		files = append(files, f.Path)
	}

	return files
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
	// SkipAgentNotification suppresses recording session links for an agent
	// hook to show later. `trace run` sets it: it reaches the push only
	// after the agent it wrapped has exited, so no hook of that agent can
	// fire again, and its own terminal already showed the links.
	SkipAgentNotification bool

	// Mode records how the session was driven, as one of the
	// aicodingsession.Mode* constants: ModeCoding for the git-hook path
	// installed by `trace init`, ModeGeneric for `trace run`. Both entry
	// points reach this same push, so the mode has to come from the caller —
	// nothing in the session itself distinguishes them. Empty means
	// ModeCoding.
	Mode string

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

	sessionMode := aicodingsession.ResolveMode(opts.Mode)

	log.Debug().Str("state_dir", store.Dir()).Str("repo_root", repoRoot).
		Bool("allow_empty", opts.AllowEmpty).Str("mode", sessionMode).Msg("trace push invoked")

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
		for sid, rec := range sessionRecords {
			// A session that has ended without contributing a commit to this
			// branch has nothing new to say; attesting it on every later push
			// republishes the same stale evidence.
			if !rec.Active {
				continue
			}
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

	// Build evidence for each session. It is written out only once the
	// attestation exists, because it records the digests of the spec materials
	// added to it first.
	type sessionEvidence struct {
		sessionID string
		evidence  *aicodingsession.Evidence
		specs     []spec.Capture
	}
	var sessions []sessionEvidence

	for sessionID, commits := range sessionCommits {
		log.Debug().Str("session", sessionID).Int("commits", len(commits)).Msg("processing session")

		provider := providerForSession(sessionRecords, sessionID)
		if provider == nil {
			log.Debug().Str("session", sessionID).Msg("no provider registered for session, skipping")
			continue
		}

		parseOpts := &trace.ParseOpts{
			SessionDir: rawDir,
			SessionID:  sessionID,
		}
		var sessionCwd, transcriptPath string
		if rec, ok := sessionRecords[sessionID]; ok && rec != nil {
			parseOpts.AgentVersion = rec.AgentVersion
			parseOpts.Model = rec.Model
			sessionCwd = rec.Cwd
			transcriptPath = rec.TranscriptPath
		}

		// Fresh copy of session data before parsing — the session-start copy
		// may be stale if more conversation happened between start and push.
		copyErr := provider.CopySessionData(store, sessionLocation(sessionID, sessionCwd, transcriptPath, repoRoot))
		if copyErr != nil {
			log.Debug().Err(copyErr).Str("session", sessionID).Msg("could not refresh session data")
		}

		result, err := provider.ParseSession(ctx, parseOpts)
		if err != nil {
			// Warn, not debug: commits carry this session in their trailer,
			// so a missing attestation fails the checks on the pull request,
			// and this is the only place that says why.
			log.Warn().Err(err).AnErr("copy_error", copyErr).Str("session", sessionID).
				Msg("could not read the transcript of an AI session named in the pushed commits; no evidence is sent for it")
			continue
		}

		// Set by the caller rather than the provider: the mode is about how the
		// session was driven, not about which agent produced it.
		result.Data.Session.Mode = sessionMode

		// The sources the agent resolved at session start, from the directory
		// the session-start hook handed it. Most sessions have none, and a
		// failure to read them is never a reason to lose the session: evidence
		// without a spec is still evidence.
		// A spec left out of the evidence is recorded as a warning, so that
		// the missing spec is visible to whoever reads the session.
		captures, specWarnings, err := spec.ReadAll(repoRoot, sessionID)
		if err != nil {
			// The error stays in the local log: its text carries local paths.
			log.Warn().Err(err).Str("session", sessionID).Msg("could not read the session spec; the session is attested without it")
			specWarnings = append(specWarnings, "the session spec was not recorded: the spec folder could not be read")
		}
		result.Data.Warnings = append(result.Data.Warnings, specWarnings...)

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

		log.Debug().Str("session", sessionID).Msg("generated evidence")

		sessions = append(sessions, sessionEvidence{
			sessionID: sessionID,
			evidence:  result,
			specs:     captures,
		})
	}

	if len(sessions) == 0 {
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

	// Add evidence for each session: its spec materials first, so that the
	// session material can record their digests, then the session itself.
	attestedSessions := make([]string, 0, len(sessions))
	// One allocator for the whole attestation: names taken from the start of
	// a session ID can repeat across sessions, and a repeated name would
	// replace an earlier material.
	names := materials.NewNameAllocator(nil)
	for _, se := range sessions {
		entries, warnings := attachSpecs(ctx, executor, newSpecRedactor(store.SpecRedactionDir(se.sessionID)), names, se.sessionID, se.specs, log)
		se.evidence.Data.Spec = entries
		se.evidence.Data.Warnings = append(se.evidence.Data.Warnings, warnings...)

		name := evidenceName(se.sessionID)
		if err := addSessionEvidence(ctx, executor, name, se.evidence); err != nil {
			// Warn, not debug: the session is left out of the attestation,
			// and this is the only place that says why.
			log.Warn().Err(err).Str("session", se.sessionID).Msg("could not add the evidence of an AI session; it is left out of the attestation")
			continue
		}
		attestedSessions = append(attestedSessions, se.sessionID)
		log.Debug().Str("session", se.sessionID).Str("name", name).Int("spec_entries", len(entries)).Msg("evidence added")
	}

	if len(attestedSessions) == 0 {
		log.Debug().Msg("no evidence successfully added, resetting attestation")
		_ = executor.Reset(ctx, AttestationResetTriggerCancelled, "no CHAINLOOP_AI_CODING_SESSION evidence added")

		return nil
	}

	// Push attestation
	log.Debug().Msg("pushing attestation")
	res, err := executor.Push(ctx)
	if err != nil {
		return fmt.Errorf("attestation push: %w", err)
	}

	// Tell the user where each session landed. The organization comes from the
	// control plane rather than opts.Organization, which is empty whenever the
	// CLI's current org is used.
	links := logAttestedSessions(log, res.UIDashboardURL, res.GetOrganization(), attestedSessions)

	// Hand the links to the agent hook that runs after this push. When the
	// push was driven by a coding agent's shell tool, the log line above is
	// captured into that tool's output rather than shown to the user, so the
	// hook is what actually puts the link in front of them. A failure here
	// costs a notification, never the attestation that already succeeded.
	//
	// The caller tells us whether to skip, rather than us inferring it from
	// on-disk state: the trace-run sentinel outlives a killed run, and
	// reading it here would silently suppress every later notification in
	// that repository.
	if opts.SkipAgentNotification {
		log.Debug().Msg("caller already showed the links; not recording them for an agent hook")
	} else if err := store.SavePendingLinks(links); err != nil {
		log.Debug().Err(err).Msg("could not record session links for the agent hook")
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

// logAttestedSessions reports one line per attested session. When the
// deployment has a UI dashboard configured the line points at the session's
// page, with the link inline so it reads as a sentence and stays clickable in
// a terminal. Without a dashboard the line still names the session, so the
// user gets confirmation of what was recorded either way.
// It returns the links it logged, so the caller can hand them to the agent
// hook that will show them to the user.
func logAttestedSessions(log zerolog.Logger, uiDashboardURL, orgName string, sessionIDs []string) []string {
	links := make([]string, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		// The link already ends in the session ID, so a session field
		// alongside it would only repeat itself in the rendered line.
		if url := buildSessionViewURL(uiDashboardURL, orgName, id); url != "" {
			log.Info().Msg(sessionLinkMessage(url))
			links = append(links, url)
			continue
		}
		log.Info().Str("session", id).Msg("Coding session attested")
	}

	return links
}

// sessionLinkMessage is the one place the user-facing wording lives, so the
// pre-push log line and the agent's notification cannot drift apart.
func sessionLinkMessage(url string) string {
	return "Coding Session Available at " + url
}

// addSessionEvidence writes one session's evidence to a temporary file and adds
// it to the attestation.
func addSessionEvidence(ctx context.Context, executor *AttestationExecutor, name string, evidence *aicodingsession.Evidence) error {
	tmpFile, err := os.CreateTemp("", "chainloop-trace-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	enc := json.NewEncoder(tmpFile)
	enc.SetIndent("", "  ")
	if err := enc.Encode(evidence); err != nil {
		_ = tmpFile.Close()

		return fmt.Errorf("write evidence: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("write evidence: %w", err)
	}

	_, err = executor.AddMaterial(ctx, name, tmpFile.Name(), "CHAINLOOP_AI_CODING_SESSION", nil)

	return err
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
		if yml := repositoryconfig.FindChainloopYML(repoRoot); yml != nil {
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
