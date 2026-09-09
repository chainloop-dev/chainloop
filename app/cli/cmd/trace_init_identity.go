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
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"

	"golang.org/x/term"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	orgPromptTitle    = "Select a Chainloop organization"
	newOrgPromptTitle = "Name for your new organization"

	// newOrgPromptReason says why a name is being asked for, since arriving at
	// this question means the user has no organization to pick from.
	newOrgPromptReason = "you do not belong to one yet, so this is the first step"

	// readOnlyOrgMarker labels an organization the caller can only view. Those
	// are shown rather than dropped — an organization missing from the list
	// reads as something gone wrong — and refused when picked. Organization
	// names are DNS-1123 labels, so no real one can end in this.
	readOnlyOrgMarker = " (read-only)"

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

// orgAPI lists the organizations the current user belongs to, and creates one
// for a user who belongs to none. It is the slice of the attestation executor
// the organization resolver needs, so that resolver can be table-tested without
// a control plane.
type orgAPI interface {
	ListOrganizations(ctx context.Context) ([]*action.MembershipItem, error)
	CreateOrganization(ctx context.Context, name string) error
}

// projectLister lists the projects visible in the selected organization, and
// is likewise the slice of the executor the project resolver needs.
type projectLister interface {
	ListProjects(ctx context.Context) (*action.TraceProjects, error)
}

// prompter asks the user to pick from a list or type a value. It keeps the
// resolution logic below independent of the terminal library, so it can be
// table-tested with a scripted fake.
// Every prompt takes a description, drawn under its title and inside the same
// frame. Context the question needs — which organization a project is being
// picked in, why a name is being asked for at all — belongs there rather than
// in a log line above it, which reads as something that happened rather than
// as part of the question. Empty means no line.
type prompter interface {
	// Select asks for one of options. validate, when set, refuses an answer and
	// keeps the prompt open, the way Input's does.
	Select(title, description string, options []string, defaultValue string, validate func(string) error) (string, error)
	// MultiSelect asks for zero or more of options, starting with defaults
	// ticked. Implementations refuse an empty submission.
	MultiSelect(title string, options, defaults []string) ([]string, error)
	Input(title, description, defaultValue string, validate func(string) error) (string, error)
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
func resolveInteractiveOrganization(ctx context.Context, api orgAPI, p prompter, fromYML, currentOrg string) (*resolvedValue, error) {
	memberships, err := api.ListOrganizations(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing your organizations: %w", err)
	}

	// labels are what the picker shows; usable holds only the ones that can be
	// picked, which is where the cursor has to start.
	labels := make([]string, 0, len(memberships))
	usable := make([]string, 0, len(memberships))
	// serverDefault is the membership the control plane marks as the current one.
	var serverDefault string

	for _, m := range memberships {
		if m.Org == nil || m.Org.Name == "" {
			continue
		}

		// An organization the caller can only view cannot hold a trace workflow,
		// since creating one is not a viewer's to do. It is shown marked rather
		// than left out: an organization that simply disappeared from the list
		// reads as something gone wrong.
		if m.Role == action.RoleViewer {
			labels = append(labels, m.Org.Name+readOnlyOrgMarker)
			continue
		}

		labels = append(labels, m.Org.Name)
		usable = append(usable, m.Org.Name)

		if m.Default {
			serverDefault = m.Org.Name
		}
	}

	if len(labels) == 0 {
		// Belonging to none is the one case where there is nothing to pick and
		// something to do about it.
		return promptNewOrganization(ctx, api, p, fromYML)
	}

	if len(usable) == 0 {
		return nil, errors.New("you can only view the organizations you belong to, ask someone with write permissions on one of their projects to set this up")
	}

	// A single entry, which the check above makes a usable one: there is nothing
	// to weigh it against, so it is not worth asking.
	if len(labels) == 1 {
		return &resolvedValue{value: usable[0], save: usable[0] != fromYML}, nil
	}

	// Preselect what the repository already pins, then what the CLI points at,
	// then what the server considers current, among the ones that can be picked.
	chosen, err := p.Select(orgPromptTitle, "", labels,
		firstPresent(usable, fromYML, currentOrg, serverDefault), refuseReadOnlyOrg)
	if err != nil {
		return nil, err
	}

	return &resolvedValue{value: chosen, save: chosen != fromYML, prompted: true}, nil
}

// refuseReadOnlyOrg keeps the picker open on an organization the caller can
// only view, saying why instead of failing later on a workflow that cannot be
// created there.
func refuseReadOnlyOrg(chosen string) error {
	name, readOnly := strings.CutSuffix(chosen, readOnlyOrgMarker)
	if !readOnly {
		return nil
	}

	return fmt.Errorf("you can only view %q, ask someone with write permissions on one of its projects to set this up", name)
}

// promptNewOrganization collects a name and creates the organization, for a
// user who belongs to none. There is nothing to select in that case and every
// later step needs an organization, so creating the first one is the only way
// forward. An instance can restrict who may create them, which leaves nothing
// to do but say so.
//
// It is the one point where trace init changes anything before the repository
// is written to. An organization left behind by an init that is then abandoned
// is the user's to keep, and is waiting for them on the next run.
func promptNewOrganization(ctx context.Context, api orgAPI, p prompter, fromYML string) (*resolvedValue, error) {
	answer, err := p.Input(newOrgPromptTitle, newOrgPromptReason, "", validateNewOrganization)
	if err != nil {
		return nil, err
	}

	name := config.SlugifyDNS1123(answer)
	if err := api.CreateOrganization(ctx, name); err != nil {
		if status.Code(err) == codes.PermissionDenied {
			return nil, fmt.Errorf("this instance only lets administrators create organizations, so ask to be invited to one: %w", err)
		}

		return nil, fmt.Errorf("creating organization %q: %w", name, err)
	}

	logger.Info().Str("organization", name).Msg("organization created")

	return &resolvedValue{value: name, save: name != fromYML, prompted: true}, nil
}

// organizationLine is the description the project prompts carry: which
// organization the projects come from, and the one a new project would be
// created in. It reports nothing when the organization is not known, which is
// the case only when no organization was ever resolved.
func organizationLine(organization string) string {
	if organization == "" {
		return ""
	}

	// A sentence rather than a label and a value: the columns of a key/value
	// line read as one more row among the options it sits above.
	return fmt.Sprintf("using the organization %s", organization)
}

// validateNewOrganization checks what a free-form name normalizes to, so a name
// the control plane would refuse is caught at the prompt. Organizations are
// named by the same rule projects are.
func validateNewOrganization(answer string) error {
	return config.ValidateDNS1123Label(config.OrganizationSubject, config.SlugifyDNS1123(answer))
}

// resolveInteractiveProject picks the project the workflow is created in, or
// collects a name for a new one. fromYML is what .chainloop.yml pins, repoDir
// the repository's directory name, used to seed a new name, and organization
// the one already settled on, shown with the question since it is what the
// projects on offer are drawn from.
func resolveInteractiveProject(ctx context.Context, lister projectLister, p prompter, fromYML, repoDir, organization string) (*resolvedValue, error) {
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
	// own directory name, no matter what .chainloop.yml holds. A project the
	// organization already has is picked from the list rather than typed, and a
	// name it does not have is one nothing was ever created under — proposing it
	// again would hand back a name as likely to be a typo as an intention.
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
			// Not in the organization at all, so it names a project that does not
			// exist rather than one that can be picked: offering it as if it were
			// real is what makes picking it fail at creation time. Nothing pinned
			// points anywhere usable, so the cursor starts on creating instead.
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
		return promptNewProject(p, organization, newProjectSeed, fromYML, readOnly)
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

	chosen, err := p.Select(title, organizationLine(organization), options, firstPresent(options, preselect), nil)
	if err != nil {
		return nil, err
	}

	if chosen == createNewProjectOption {
		return promptNewProject(p, organization, newProjectSeed, fromYML, readOnly)
	}

	return &resolvedValue{value: chosen, save: chosen != fromYML, prompted: true}, nil
}

// promptNewProject collects a free-form project name and returns it normalized
// to the DNS-1123 label the control plane stores. A name that normalizes to one
// of the existing projects simply resolves to that project, so readOnly names
// are refused: typing one is the same dead end as picking it from the list.
func promptNewProject(p prompter, organization, defaultName, fromYML string, readOnly []string) (*resolvedValue, error) {
	answer, err := p.Input(newProjectPromptTitle, organizationLine(organization), defaultName, newProjectValidator(readOnly))
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
		if err := config.ValidateDNS1123Label(config.ProjectSubject, name); err != nil {
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
