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
	"cmp"
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
	orgPromptTitle        = "Chainloop organization for this repository"
	projectPromptTitle    = "Chainloop project for this repository"
	newProjectPromptTitle = "Name for the new project"

	// createNewProjectOption is the first entry of the project picker.
	createNewProjectOption = "Create a new project…"
)

// errAborted is returned when the user dismisses a prompt. It stops `trace
// init` before anything is created or written, so the repository is left
// untouched.
var errAborted = errors.New("aborted")

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
	ListProjects(ctx context.Context) ([]string, error)
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
	visible, err := lister.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the projects you can see: %w", err)
	}

	// Nothing to choose from, so go straight to naming one.
	if len(visible) == 0 {
		return promptNewProject(p, cmp.Or(fromYML, config.SlugifyDNS1123(repoDir)), fromYML)
	}

	// A project pinned in .chainloop.yml is offered even when the listing does
	// not carry it: the user may only be able to see it through this repository,
	// or it may not exist yet. Either way it is still this repository's project.
	if fromYML != "" && !slices.Contains(visible, fromYML) {
		visible = append(visible, fromYML)
	}

	options := append([]string{createNewProjectOption}, visible...)

	chosen, err := p.Select(projectPromptTitle, options, cmp.Or(fromYML, createNewProjectOption))
	if err != nil {
		return nil, err
	}

	if chosen != createNewProjectOption {
		return &resolvedValue{value: chosen, save: chosen != fromYML, prompted: true}, nil
	}

	// They asked for a new project, so seed the name from the repository rather
	// than from the project they are moving away from.
	return promptNewProject(p, config.SlugifyDNS1123(repoDir), fromYML)
}

// promptNewProject collects a free-form project name and returns it normalized
// to the DNS-1123 label the control plane stores. A name that normalizes to one
// of the existing projects simply resolves to that project.
func promptNewProject(p prompter, defaultName, fromYML string) (*resolvedValue, error) {
	answer, err := p.Input(newProjectPromptTitle, defaultName, validateNewProjectName)
	if err != nil {
		return nil, err
	}

	name := config.SlugifyDNS1123(answer)

	return &resolvedValue{value: name, save: name != fromYML, prompted: true}, nil
}

// validateNewProjectName checks what a free-form name normalizes to, so a name
// that cannot become a valid project is refused at the prompt instead of by the
// control plane a moment later.
func validateNewProjectName(answer string) error {
	return config.ValidateDNS1123Label(config.SlugifyDNS1123(answer))
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
