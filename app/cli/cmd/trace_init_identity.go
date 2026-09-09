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

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"

	"golang.org/x/term"
)

const (
	orgPromptTitle = "Select a Chainloop organization"

	// The project picker offers creating one only to a caller allowed to, so it
	// has a title for each case: promising an action that is not on the list
	// would be the same dead end as offering the action itself.
	projectPromptTitle       = "Select or create a Chainloop project"
	selectProjectPromptTitle = "Select a Chainloop project"

	newProjectPromptTitle = "Name for the new project"

	// createNewProjectOption closes the project picker, below the projects
	// themselves. huh has no separator or grouping in a list — every entry is a
	// label and a value, drawn alike — so the "+" is what sets an action apart
	// from the names it sits under.
	createNewProjectOption = "+ Create a new project…"
)

// errAborted is returned when the user dismisses a prompt. It stops `trace
// init` before anything is created or written, so the repository is left
// untouched.
var errAborted = errors.New("aborted")

// stopIfAborted turns a dismissed prompt into a clean stop and passes every
// other error through. Dismissing a question is a decision, not a failure, and
// nothing has been created or written by the time one can be dismissed, so
// reporting it as an error would be both noisy and wrong.
func stopIfAborted(err error) error {
	if !errors.Is(err, errAborted) {
		return err
	}

	logger.Info().Msg("aborted, nothing was changed")

	return nil
}

// ciSignals are environment variables that mean the CLI is running inside a CI
// system, where a prompt would hang the build. CI covers almost every provider;
// the rest are for the ones that do not set it.
var ciSignals = []string{
	"CI",
	"CONTINUOUS_INTEGRATION",
	"BUILD_NUMBER",
	"BUILD_BUILDID",
	"TEAMCITY_VERSION",
	"JENKINS_URL",
}

// noPromptEnvVar lets a user with a real terminal turn prompting off, for
// wrappers and automation that run the CLI attached to a pseudo-terminal.
const noPromptEnvVar = "CHAINLOOP_NO_PROMPT"

// orgLister lists the organizations the current user belongs to. It is the
// slice of the attestation executor the organization resolver needs, so that
// resolver can be table-tested without a control plane.
type orgLister interface {
	ListOrganizations(ctx context.Context) ([]*action.MembershipItem, error)
}

// projectLister lists the projects visible in the selected organization, and
// is likewise the slice of the executor the project resolver needs.
type projectLister interface {
	ListProjects(ctx context.Context) (*action.TraceProjects, error)
}

// prompter asks the user to pick from a list or type a value. It keeps the
// resolution logic below independent of the terminal library, so it can be
// table-tested with a scripted fake.
type prompter interface {
	Select(title string, options []string, defaultValue string) (string, error)
	// MultiSelect asks for zero or more of options, starting with defaults
	// ticked. Implementations refuse an empty submission.
	MultiSelect(title string, options, defaults []string) ([]string, error)
	Input(title, defaultValue string, validate func(string) error) (string, error)
}

// resolvedValue is one setting the interactive path settled on.
type resolvedValue struct {
	value string
	// save is true when the value differs from what .chainloop.yml holds and so
	// has to be written back. An absent value in the file counts as different,
	// which leaves an interactive init with a fully self-describing repository.
	save bool
	// prompted is false when the value was settled without asking, so the caller
	// can report what was picked on the user's behalf.
	prompted bool
}

// traceInitCanPrompt reports whether `trace init` has a user to ask, wiring
// isInteractive to the real environment and terminals.
func traceInitCanPrompt() bool {
	return isInteractive(os.LookupEnv,
		term.IsTerminal(int(os.Stdin.Fd())), term.IsTerminal(int(os.Stderr.Fd())))
}

// isInteractive reports whether the session can show a prompt: both streams the
// prompt uses are terminals, the terminal can render one, and this is not a CI
// run. It takes its inputs so the decision can be unit tested, the same way
// colorSupported does.
func isInteractive(lookupEnv func(string) (string, bool), stdinIsTerminal, stderrIsTerminal bool) bool {
	if envEnabled(lookupEnv, noPromptEnvVar) {
		return false
	}

	// The prompt reads from stdin and draws on stderr; either one redirected
	// means there is nobody to answer.
	if !stdinIsTerminal || !stderrIsTerminal {
		return false
	}

	if v, ok := lookupEnv("TERM"); ok && v == "dumb" {
		return false
	}

	for _, key := range ciSignals {
		if envEnabled(lookupEnv, key) {
			return false
		}
	}

	return true
}

// envEnabled reports whether an environment variable is set to something that
// means "yes". Present but empty, "0" or "false" is treated as unset, since
// developers do set CI=false locally.
func envEnabled(lookupEnv func(string) (string, bool), key string) bool {
	v, ok := lookupEnv(key)
	return ok && v != "" && v != "0" && v != "false"
}

// resolveInteractiveOrganization picks the organization the trace attestations
// land in. fromYML is what .chainloop.yml pins, currentOrg the one the CLI is
// pointing at; both may be empty.
func resolveInteractiveOrganization(ctx context.Context, lister orgLister, p prompter, fromYML, currentOrg string) (*resolvedValue, error) {
	memberships, err := lister.ListOrganizations(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing your organizations: %w", err)
	}

	names := make([]string, 0, len(memberships))
	// serverDefault is the membership the control plane marks as the current one.
	var serverDefault string
	for _, m := range memberships {
		if m.Org == nil || m.Org.Name == "" {
			continue
		}

		names = append(names, m.Org.Name)
		if m.Default {
			serverDefault = m.Org.Name
		}
	}

	switch len(names) {
	case 0:
		return nil, errors.New("you do not belong to any organization, create one with `chainloop organization create`")
	case 1:
		return &resolvedValue{value: names[0], save: names[0] != fromYML}, nil
	}

	// Preselect what the repository already pins, then what the CLI points at,
	// then what the server considers current.
	chosen, err := p.Select(orgPromptTitle, names, firstPresent(names, fromYML, currentOrg, serverDefault))
	if err != nil {
		return nil, err
	}

	return &resolvedValue{value: chosen, save: chosen != fromYML, prompted: true}, nil
}

// resolveInteractiveProject picks the project the workflow is created in, or
// collects a name for a new one. fromYML is what .chainloop.yml pins and
// repoDir the repository's directory name, used to seed a new name.
func resolveInteractiveProject(ctx context.Context, lister projectLister, p prompter, fromYML, repoDir string) (*resolvedValue, error) {
	listing, err := lister.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the projects you can see: %w", err)
	}

	visible := listing.Projects

	// Only the projects the caller can add a workflow to are worth offering;
	// picking any other one would fail at creation time. The rest are still
	// worth knowing about, to refuse one that gets typed in by name.
	writable := make([]string, 0, len(visible))
	readOnly := make([]string, 0, len(visible))

	for _, project := range visible {
		if project.CanCreateWorkflow {
			writable = append(writable, project.Name)
			continue
		}

		readOnly = append(readOnly, project.Name)
	}

	// newProjectSeed is what the new-project prompt starts from: the repository's
	// own directory name, whatever .chainloop.yml holds. A project the
	// organization already has is picked from the list rather than typed, and a
	// name it does not have is one nothing was ever created under — proposing it
	// again would hand back a name that is as likely to be a typo as an
	// intention.
	newProjectSeed := config.SlugifyDNS1123(repoDir)

	// pinnedIsMissing records that .chainloop.yml names a project the
	// organization does not have, which decides where the cursor starts.
	var pinnedIsMissing bool

	// The listing is the source of truth: what .chainloop.yml names is only worth
	// offering if the organization actually has it. Anything else it holds is
	// stale, names a project in another organization, or was never created.
	if fromYML != "" && !slices.Contains(writable, fromYML) {
		if slices.Contains(readOnly, fromYML) {
			logger.Warn().Str("project", fromYML).
				Msg("you cannot create a workflow in the project this repository points at, pick another one")
		} else {
			// Not in the organization at all, so it is a name for a project that does
			// not exist rather than one that can be picked. Offering it as if it were
			// real is what makes selecting it fail on a project the caller may not be
			// allowed to create. Nothing points anywhere usable, so the cursor starts
			// on creating one; the name is asked for rather than carried over.
			pinnedIsMissing = true

			logger.Debug().Str("project", fromYML).
				Msg("the project this repository points at does not exist in this organization")
		}
	}

	// Creating one is a permission of the organization role, not of any project:
	// an organization contributor administering every project it can see still
	// cannot create another. With nothing to pick either, there is no way
	// forward, so say so instead of asking.
	if !listing.CanCreateProject && len(writable) == 0 {
		return nil, errors.New("you cannot create projects in this organization, and none of the projects you can see accepts a new workflow; ask an administrator for access to one")
	}

	// Nothing to choose from, so go straight to naming one rather than showing a
	// list holding nothing but the create entry.
	if len(writable) == 0 {
		return promptNewProject(p, newProjectSeed, fromYML, readOnly)
	}

	// The create entry is only offered to a role that may act on it; offering it
	// anyway would fail on the name they just typed. It goes last, under the
	// projects, since picking one of those is the common answer.
	title, options := selectProjectPromptTitle, writable
	if listing.CanCreateProject {
		title = projectPromptTitle
		options = append(slices.Clone(writable), createNewProjectOption)
	}

	// The cursor starts on what the repository pins, and on creating when what it
	// pins does not exist, since that name is what it is asking to be created.
	preselect := fromYML
	if pinnedIsMissing {
		preselect = createNewProjectOption
	}

	chosen, err := p.Select(title, options, firstPresent(options, preselect))
	if err != nil {
		return nil, err
	}

	if chosen == createNewProjectOption {
		return promptNewProject(p, newProjectSeed, fromYML, readOnly)
	}

	return &resolvedValue{value: chosen, save: chosen != fromYML, prompted: true}, nil
}

// promptNewProject collects a free-form project name and returns it normalized
// to the DNS-1123 label the control plane stores. A name that normalizes to one
// of the existing projects simply resolves to that project, so readOnly names
// are refused: typing one is the same dead end as picking it from the list.
func promptNewProject(p prompter, defaultName, fromYML string, readOnly []string) (*resolvedValue, error) {
	answer, err := p.Input(newProjectPromptTitle, defaultName, newProjectValidator(readOnly))
	if err != nil {
		return nil, err
	}

	name := config.SlugifyDNS1123(answer)

	return &resolvedValue{value: name, save: name != fromYML, prompted: true}, nil
}

// newProjectValidator checks what a free-form name normalizes to, so a name
// that cannot become a valid project is refused at the prompt instead of by the
// control plane a moment later. readOnly are the projects the caller can see but
// not write to, refused here for the same reason they are not offered.
func newProjectValidator(readOnly []string) func(string) error {
	return func(answer string) error {
		name := config.SlugifyDNS1123(answer)
		if err := config.ValidateDNS1123Label(name); err != nil {
			return err
		}

		if slices.Contains(readOnly, name) {
			return fmt.Errorf("you can only view the project %q, so a workflow cannot be created in it", name)
		}

		return nil
	}
}

// firstPresent returns the first candidate that appears in options, falling
// back to the first option.
func firstPresent(options []string, candidates ...string) string {
	for _, c := range candidates {
		if c != "" && slices.Contains(options, c) {
			return c
		}
	}

	return options[0]
}
