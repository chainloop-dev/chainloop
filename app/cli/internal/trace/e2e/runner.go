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

package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// Filter limits a manual run to some harnesses, models or features (R-009).
// An empty list matches all.
type Filter struct {
	Harnesses []string
	Models    []string
	Features  []string
}

// Empty reports whether the filter matches all cells. Only a run with an empty
// filter is a full run.
func (f Filter) Empty() bool {
	return len(f.Harnesses) == 0 && len(f.Models) == 0 && len(f.Features) == 0
}

func matches(list []string, v string) bool {
	return len(list) == 0 || slices.Contains(list, v)
}

// Runner expands the expected matrix into scenario sessions, runs them, and
// builds the result matrix.
type Runner struct {
	Expected  *Expected
	Config    *Config
	Harnesses []Harness
	Filter    Filter
	// ChainloopBin is the chainloop CLI under test.
	ChainloopBin string
	CLIVersion   string
	// WorkDir holds the workspaces and the harness output of each session.
	WorkDir string
	Logf    func(format string, args ...any)
}

// sessionResult is one run of a scenario. err is set when the session could
// not run, which is a suite problem and not a feature failure.
type sessionResult struct {
	session *Session
	err     error
}

// plannedCell is a cell to test, with the repeat of its feature.
type plannedCell struct {
	cell    Cell
	feature Feature
	repeat  Repeat
}

// Run runs the suite and returns the result matrix.
func (r *Runner) Run(ctx context.Context) (*Result, error) {
	res := &Result{StartedAt: time.Now().UTC(), CLIVersion: r.CLIVersion, Full: r.Filter.Empty()}

	for _, h := range r.Harnesses {
		if !matches(r.Filter.Harnesses, h.Column()) {
			continue
		}
		if !slices.Contains(r.Expected.Harnesses, h.Column()) {
			return nil, fmt.Errorf("harness %q has no column in the feature table", h.Column())
		}
		hc, ok := r.Config.Harnesses[h.Column()]
		if !ok {
			return nil, fmt.Errorf("harness %q has no entry in the suite config", h.Column())
		}

		version, versionErr := h.Version(ctx)
		if versionErr == nil && version != hc.Version {
			r.Logf("%s: installed version %s is not the pinned version %s", h.Column(), version, hc.Version)
		}

		for _, model := range hc.Models {
			if !matches(r.Filter.Models, model) {
				continue
			}
			cells, err := r.runHarnessModel(ctx, h, version, versionErr, model)
			if err != nil {
				return nil, err
			}
			res.Cells = append(res.Cells, cells...)
		}
	}

	return res, nil
}

// runHarnessModel tests all the cells of one harness and one model.
func (r *Runner) runHarnessModel(ctx context.Context, h Harness, version string, versionErr error, model string) ([]Cell, error) {
	var (
		cells   []Cell
		planned []plannedCell
		runs    = make(map[string]int) // scenario name to the runs it needs
	)

	for _, name := range r.Expected.Features {
		if !matches(r.Filter.Features, name) {
			continue
		}
		support, _ := r.Expected.Support(name, h.Column())
		c := Cell{Feature: name, Harness: h.Column(), HarnessVersion: version, Model: model, Expected: support}

		def, hasDef := features[name]
		switch {
		case !support.Tested():
			c.Status, c.Reason = StatusSkipped, "the feature table marks it "+string(support)
		case !hasDef || (!def.ControlPlane && def.NotCovered == "" && len(def.Checks) == 0):
			c.Status, c.Reason = StatusNoCheck, "no check is registered for this feature"
		case def.ControlPlane:
			c.Status, c.Reason = StatusNotCovered, "the feature needs a control plane"
		case def.NotCovered != "":
			c.Status, c.Reason = StatusNotCovered, def.NotCovered
		case versionErr != nil:
			c.Status, c.Reason = StatusError, versionErr.Error()
		default:
			repeat := r.Config.repeatFor(name, def.ModelDriven)
			planned = append(planned, plannedCell{cell: c, feature: def, repeat: repeat})
			for _, chk := range def.Checks {
				runs[chk.Scenario] = max(runs[chk.Scenario], repeat.Runs)
			}
			continue
		}
		cells = append(cells, c)
	}

	sessions, err := r.runScenarios(ctx, h, model, runs)
	if err != nil {
		return nil, err
	}

	for _, p := range planned {
		cells = append(cells, evaluate(p, sessions))
	}

	// Keep the order of the feature table.
	order := make(map[string]int, len(r.Expected.Features))
	for i, f := range r.Expected.Features {
		order[f] = i
	}
	slices.SortStableFunc(cells, func(a, b Cell) int { return order[a.Feature] - order[b.Feature] })

	return cells, nil
}

// runScenarios runs each scenario the given number of times, in parallel.
func (r *Runner) runScenarios(ctx context.Context, h Harness, model string, runs map[string]int) (map[string][]sessionResult, error) {
	var (
		mu  sync.Mutex
		out = make(map[string][]sessionResult, len(runs))
	)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(r.Config.Parallel)

	for name, n := range runs {
		sc, ok := scenarios[name]
		if !ok {
			return nil, fmt.Errorf("unknown scenario %q", name)
		}
		out[name] = make([]sessionResult, n)
		for i := range n {
			g.Go(func() error {
				s, err := r.runScenario(gctx, h, model, sc, i)
				if err != nil {
					r.Logf("%s / %s / %s #%d: %v", h.Column(), model, sc.Name, i+1, err)
				}
				mu.Lock()
				out[name][i] = sessionResult{session: s, err: err}
				mu.Unlock()
				return nil
			})
		}
	}
	_ = g.Wait()

	return out, ctx.Err()
}

// runScenario runs one session of a scenario: it creates the workspace, runs
// the harness, pushes the commit of the agent, and reads the stored session.
func (r *Runner) runScenario(ctx context.Context, h Harness, model string, sc Scenario, i int) (*Session, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.Config.ScenarioTimeout))
	defer cancel()

	base := filepath.Join(r.WorkDir, dirName(h.Column()), dirName(model), sc.Name, strconv.Itoa(i+1))
	r.Logf("%s / %s / %s #%d: start in %s", h.Column(), model, sc.Name, i+1, base)

	ws, err := NewWorkspace(ctx, base, r.ChainloopBin, h, sc)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	if sc.NoAgent {
		return &Session{Scenario: sc.Name, Model: model, Harness: h, Workspace: ws, Output: &RunOutput{}, Checkout: ws.Dir}, nil
	}

	req := RunRequest{Dir: ws.Dir, Model: model, Prompt: sc.Prompt, Env: ws.Env, Images: sc.Images}
	if sc.WrapTraceRun {
		req.Wrapper = []string{r.ChainloopBin, "trace", "run", "--local", h.TraceFlag(), "--"}
	}
	out, runErr := h.Run(ctx, req)
	if out != nil {
		// The harness output stays next to the workspace to debug a failure.
		if err := os.WriteFile(filepath.Join(base, "harness-output.jsonl"), out.Raw, 0o600); err != nil {
			return nil, err
		}
	}
	if runErr != nil {
		return nil, runErr
	}

	s := &Session{Scenario: sc.Name, Model: model, Harness: h, Workspace: ws, Output: out, Checkout: ws.Dir}

	// The agent pushes in the session, as a user does. The pre-push hook
	// stores the session in local storage mode. The suite does not push after
	// the session: the session end removes the captured specs, so a later push
	// stores the session with no spec. A session that did not push stores
	// nothing, and its checks fail.
	checkout := ws.Dir
	if sc.Checkout != "" {
		checkout = filepath.Join(base, sc.Checkout)
		if _, err := os.Stat(checkout); err != nil {
			// The agent did not create the checkout, so it stored nothing.
			r.Logf("%s / %s / %s #%d: no checkout %s", h.Column(), model, sc.Name, i+1, sc.Checkout)
			return s, nil
		}
	}

	s.Checkout = checkout
	if sc.FollowUp != nil {
		if err := sc.FollowUp(ctx, ws, checkout); err != nil {
			return nil, fmt.Errorf("follow-up of the session: %w", err)
		}
	}
	if s.HeadMessage, err = ws.run(ctx, checkout, "git", "log", "-1", "--format=%B"); err != nil {
		return nil, err
	}
	head, err := ws.run(ctx, checkout, "git", "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	s.HeadSHA = strings.TrimSpace(head)

	stored, err := StoredSessions(checkout)
	if err != nil {
		return nil, err
	}
	switch len(stored) {
	case 0:
	case 1:
		s.Stored = &stored[0]
	default:
		return nil, fmt.Errorf("the push stored %d sessions, want 1", len(stored))
	}

	r.Logf("%s / %s / %s #%d: done", h.Column(), model, sc.Name, i+1)
	return s, nil
}

// errNoStoredSession is the failure of a check when the push stored no
// session, for example because the agent did not commit or push.
var errNoStoredSession = errors.New("the push stored no session evidence; inspect harness-output.jsonl and the workspace git history")

// evaluate runs the checks of a planned cell on its sessions and sets its
// status and pass rate.
func evaluate(p plannedCell, sessions map[string][]sessionResult) Cell {
	c := p.cell
	c.Runs = p.repeat.Runs

	var reasons []string
	for i := range p.repeat.Runs {
		ok := true
		for _, chk := range p.feature.Checks {
			sr := sessions[chk.Scenario][i]
			if sr.err != nil {
				c.Status, c.Reason = StatusError, fmt.Sprintf("scenario %s could not run: %v", chk.Scenario, sr.err)
				return c
			}

			// A scenario with no agent stores no session. Its checks read the
			// workspace.
			var err error
			if sr.session.Stored == nil && !scenarios[chk.Scenario].NoAgent {
				err = errNoStoredSession
			} else {
				err = chk.Verify(sr.session)
			}
			if errors.Is(err, errNotCovered) {
				c.Status, c.Reason = StatusNotCovered, err.Error()
				return c
			}
			if err != nil {
				ok = false
				reasons = append(reasons, fmt.Sprintf("run %d: %v", i+1, err))
			}
		}
		if ok {
			c.Passes++
		}
	}

	c.Status = StatusFail
	if c.Passes >= p.repeat.Threshold {
		c.Status = StatusPass
	}
	c.Reason = strings.Join(reasons, "; ")
	return c
}

// dirName turns a harness or model name into a directory name.
func dirName(s string) string {
	return strings.NewReplacer(" ", "-", "/", "-", ":", "-").Replace(strings.ToLower(s))
}
