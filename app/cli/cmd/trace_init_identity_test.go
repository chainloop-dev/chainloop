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
	"slices"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- fakes -------------------------------------------------------------------

// fakeOrgAPI serves a canned membership list and records what the resolver
// asked to have created.
type fakeOrgAPI struct {
	orgs []*action.MembershipItem
	err  error

	// createErr is what creating an organization fails with, and created
	// records the name it was asked for.
	createErr error
	created   string
}

func (f *fakeOrgAPI) ListOrganizations(context.Context) ([]*action.MembershipItem, error) {
	return f.orgs, f.err
}

func (f *fakeOrgAPI) CreateOrganization(_ context.Context, name string) error {
	f.created = name

	return f.createErr
}

// fakeProjectLister serves a canned listing. projects are the ones the caller
// may create a workflow in, readOnly the ones it can only see. cannotCreate
// inverts the default so the common case stays untyped in the table.
type fakeProjectLister struct {
	projects     []string
	readOnly     []string
	cannotCreate bool
	err          error
}

func (f *fakeProjectLister) ListProjects(context.Context) (*action.TraceProjects, error) {
	if f.err != nil {
		return nil, f.err
	}

	out := make([]*action.TraceProject, 0, len(f.projects)+len(f.readOnly))
	for _, name := range f.projects {
		out = append(out, &action.TraceProject{Name: name, CanCreateWorkflow: true})
	}

	for _, name := range f.readOnly {
		out = append(out, &action.TraceProject{Name: name})
	}

	return &action.TraceProjects{Projects: out, CanCreateProject: !f.cannotCreate}, nil
}

// recordedPrompt captures what a prompt was shown, so tests can assert on the
// options and the preselected default rather than only on the outcome.
type recordedPrompt struct {
	title string
	// description is the line drawn under the title, inside the same frame.
	description  string
	options      []string
	defaultValue string
	// defaults is the preselected set for a multi-select.
	defaults []string
}

// fakePrompter answers with scripted values. Input runs the real validator
// first, the way a terminal prompt would, so validation is covered through the
// same seam the huh implementation uses.
type fakePrompter struct {
	selectAnswer      string
	selectErr         error
	multiSelectAnswer []string
	multiSelectErr    error
	inputAnswer       string
	inputErr          error

	selects      []recordedPrompt
	multiSelects []recordedPrompt
	inputs       []recordedPrompt
}

func (f *fakePrompter) MultiSelect(title string, options, defaults []string) ([]string, error) {
	f.multiSelects = append(f.multiSelects, recordedPrompt{title: title, options: options, defaults: defaults})
	if f.multiSelectErr != nil {
		return nil, f.multiSelectErr
	}

	return f.multiSelectAnswer, nil
}

func (f *fakePrompter) Select(title, description string, options []string, defaultValue string, validate func(string) error) (string, error) {
	f.selects = append(f.selects, recordedPrompt{title: title, description: description, options: options, defaultValue: defaultValue})
	if f.selectErr != nil {
		return "", f.selectErr
	}

	// Run the validator the way a terminal prompt would, so what it refuses is
	// covered through the same seam the huh implementation uses.
	if validate != nil {
		if err := validate(f.selectAnswer); err != nil {
			return "", err
		}
	}

	return f.selectAnswer, nil
}

func (f *fakePrompter) Input(title, description, defaultValue string, validate func(string) error) (string, error) {
	f.inputs = append(f.inputs, recordedPrompt{title: title, description: description, defaultValue: defaultValue})
	if f.inputErr != nil {
		return "", f.inputErr
	}

	if validate != nil {
		if err := validate(f.inputAnswer); err != nil {
			return "", err
		}
	}

	return f.inputAnswer, nil
}

func membership(name string, isDefault bool) *action.MembershipItem {
	return &action.MembershipItem{Default: isDefault, Org: &action.OrgItem{Name: name}}
}

// viewerMembership is one the caller can only read, where no workflow can be
// created and so no tracing can be set up.
func viewerMembership(name string, isDefault bool) *action.MembershipItem {
	m := membership(name, isDefault)
	m.Role = action.RoleViewer

	return m
}

// Fixtures the table cases share.
const (
	// nameWithNothingUsable normalizes to an empty string, so it is refused
	// wherever a name is asked for.
	nameWithNothingUsable = "!!!"

	orgAcme       = "acme"
	orgGlobex     = "globex"
	projectAPI    = "backend-api"
	repoDirMyRepo = "My Repo"
)

// --- isInteractive -----------------------------------------------------------

func TestIsInteractive(t *testing.T) {
	testCases := []struct {
		name   string
		env    map[string]string
		stdin  bool
		stderr bool
		want   bool
	}{
		{name: "both streams are terminals and no CI signal", stdin: true, stderr: true, want: true},
		{name: "stdin is not a terminal", stdin: false, stderr: true, want: false},
		{name: "stderr is not a terminal", stdin: true, stderr: false, want: false},
		{name: "neither is a terminal", want: false},
		{
			name:  "CI is set, even with a pseudo-terminal",
			env:   map[string]string{"CI": "true"},
			stdin: true, stderr: true, want: false,
		},
		{
			name:  "CI explicitly set to false is not CI",
			env:   map[string]string{"CI": "false"},
			stdin: true, stderr: true, want: true,
		},
		{
			name:  "CI set to an empty value is not CI",
			env:   map[string]string{"CI": ""},
			stdin: true, stderr: true, want: true,
		},
		// Providers that do not set CI are covered by TestIsInteractiveCISignals.
		{
			name:  "a dumb terminal cannot render a prompt",
			env:   map[string]string{"TERM": "dumb"},
			stdin: true, stderr: true, want: false,
		},
		{
			name:  "the explicit opt-out wins over a real terminal",
			env:   map[string]string{"CHAINLOOP_NO_PROMPT": "1"},
			stdin: true, stderr: true, want: false,
		},
		{
			name:  "the opt-out set to 0 does not opt out",
			env:   map[string]string{"CHAINLOOP_NO_PROMPT": "0"},
			stdin: true, stderr: true, want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lookupEnv := func(key string) (string, bool) {
				v, ok := tc.env[key]
				return v, ok
			}
			assert.Equal(t, tc.want, isInteractive(lookupEnv, tc.stdin, tc.stderr))
		})
	}
}

// TestIsInteractiveCISignals covers every provider in the list, so adding one
// without it actually suppressing prompting fails here.
func TestIsInteractiveCISignals(t *testing.T) {
	for _, key := range ciSignals {
		t.Run(key, func(t *testing.T) {
			lookupEnv := func(k string) (string, bool) {
				if k == key {
					return "1", true
				}

				return "", false
			}
			assert.False(t, isInteractive(lookupEnv, true, true), "%s must disable prompting", key)
		})
	}
}

// --- organization ------------------------------------------------------------

func TestResolveInteractiveOrganization(t *testing.T) {
	testCases := []struct {
		name       string
		orgs       []*action.MembershipItem
		fromYML    string
		currentOrg string
		answer     string
		// inputAnswer is what the user types when there is no organization to
		// pick and one has to be created.
		inputAnswer string
		// createErr is what creating the organization fails with.
		createErr error
		wantValue string
		wantSave  bool
		// wantDefault is the preselected entry; empty means no prompt is expected
		wantDefault string
		// wantCreated is the organization the resolver asked to have created.
		wantCreated string
		wantErr     string
	}{
		{
			// Nothing to select, so the only way forward is the first one.
			name:        "belonging to no organization creates one",
			orgs:        nil,
			inputAnswer: "My First Org",
			wantValue:   "my-first-org",
			wantSave:    true,
			wantCreated: "my-first-org",
		},
		{
			// An instance can restrict creating organizations to its
			// administrators, which leaves a user who belongs to none with
			// nothing to do but ask.
			name:        "an instance that forbids creating organizations says so",
			inputAnswer: "my-org",
			createErr:   status.Error(codes.PermissionDenied, "restricted to instance admins"),
			wantCreated: "my-org",
			wantErr:     "ask to be invited",
		},
		{
			name:        "a creation failure is returned",
			inputAnswer: "my-org",
			createErr:   errors.New("boom"),
			wantCreated: "my-org",
			wantErr:     "boom",
		},
		{
			// A name the control plane would refuse is caught at the prompt, so
			// nothing is attempted with it.
			name:        "an unusable new name is refused before creating",
			inputAnswer: nameWithNothingUsable,
			wantErr:     "organization name cannot be empty",
		},
		{
			name:      "a single organization is used without prompting",
			orgs:      []*action.MembershipItem{membership(orgAcme, true)},
			wantValue: orgAcme,
			wantSave:  true,
		},
		{
			name:      "a single organization already pinned needs no save",
			orgs:      []*action.MembershipItem{membership(orgAcme, true)},
			fromYML:   orgAcme,
			wantValue: orgAcme,
			wantSave:  false,
		},
		{
			name:        "several organizations preselect the pinned one",
			orgs:        []*action.MembershipItem{membership(orgAcme, true), membership(orgGlobex, false)},
			fromYML:     orgGlobex,
			currentOrg:  orgAcme,
			answer:      orgGlobex,
			wantValue:   orgGlobex,
			wantSave:    false,
			wantDefault: orgGlobex,
		},
		{
			name:        "with nothing pinned the CLI's current organization is preselected",
			orgs:        []*action.MembershipItem{membership(orgAcme, false), membership(orgGlobex, true)},
			currentOrg:  orgAcme,
			answer:      orgAcme,
			wantValue:   orgAcme,
			wantSave:    true,
			wantDefault: orgAcme,
		},
		{
			name:        "with no current organization the default membership is preselected",
			orgs:        []*action.MembershipItem{membership(orgAcme, false), membership(orgGlobex, true)},
			answer:      orgGlobex,
			wantValue:   orgGlobex,
			wantSave:    true,
			wantDefault: orgGlobex,
		},
		{
			name:        "a pinned organization the user no longer belongs to falls back to the current one",
			orgs:        []*action.MembershipItem{membership(orgAcme, true), membership(orgGlobex, false)},
			fromYML:     "left-this-one",
			currentOrg:  orgAcme,
			answer:      orgAcme,
			wantValue:   orgAcme,
			wantSave:    true,
			wantDefault: orgAcme,
		},
		{
			name:        "picking a different organization than the pinned one saves it",
			orgs:        []*action.MembershipItem{membership(orgAcme, true), membership(orgGlobex, false)},
			fromYML:     orgAcme,
			currentOrg:  orgAcme,
			answer:      orgGlobex,
			wantValue:   orgGlobex,
			wantSave:    true,
			wantDefault: orgAcme,
		},
		{
			// Shown so it is clear the organization is still there, and why it
			// cannot be used, rather than going missing from the list.
			name:        "an organization the user can only view is marked and refused",
			orgs:        []*action.MembershipItem{membership(orgAcme, true), viewerMembership(orgGlobex, false)},
			answer:      orgGlobex + readOnlyOrgMarker,
			wantDefault: orgAcme,
			wantErr:     "you can only view",
		},
		{
			// The cursor cannot start on an entry that would be refused, even
			// when the server calls it the current one.
			name:        "a viewer organization is never the preselected one",
			orgs:        []*action.MembershipItem{viewerMembership(orgAcme, true), membership(orgGlobex, false)},
			answer:      orgGlobex,
			wantValue:   orgGlobex,
			wantSave:    true,
			wantDefault: orgGlobex,
		},
		{
			// Not the same as belonging to none: there is nothing to create here,
			// only someone to ask.
			name:    "only viewer organizations leaves nothing to set up",
			orgs:    []*action.MembershipItem{viewerMembership(orgAcme, true), viewerMembership(orgGlobex, false)},
			wantErr: "write permissions on one of their projects",
		},
		{
			// The one usable organization is not the only entry, so the picker is
			// still shown: the viewer one has to be visible to be understood.
			name:        "a single usable organization beside a viewer one still prompts",
			orgs:        []*action.MembershipItem{membership(orgAcme, false), viewerMembership(orgGlobex, false)},
			answer:      orgAcme,
			wantValue:   orgAcme,
			wantSave:    true,
			wantDefault: orgAcme,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := &fakePrompter{selectAnswer: tc.answer, inputAnswer: tc.inputAnswer}
			api := &fakeOrgAPI{orgs: tc.orgs, createErr: tc.createErr}
			got, err := resolveInteractiveOrganization(context.Background(), api, p, tc.fromYML, tc.currentOrg)

			// Asserted before the error check too, so a case that must not reach
			// the control plane says so.
			assert.Equal(t, tc.wantCreated, api.created, "organization created")

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantValue, got.value)
			assert.Equal(t, tc.wantSave, got.save)

			if tc.wantCreated != "" {
				assert.Empty(t, p.selects, "there was nothing to pick from")
				require.Len(t, p.inputs, 1, "the name should have been asked for")
				assert.True(t, got.prompted)
				return
			}

			if tc.wantDefault == "" {
				assert.Empty(t, p.selects, "no prompt should have been shown")
				assert.False(t, got.prompted, "the value was not prompted for")
				return
			}

			assert.True(t, got.prompted)
			require.Len(t, p.selects, 1)
			assert.Equal(t, tc.wantDefault, p.selects[0].defaultValue)
			assert.Len(t, p.selects[0].options, len(tc.orgs))
		})
	}
}

func TestResolveInteractiveOrganizationErrors(t *testing.T) {
	t.Run("a listing failure is returned, not swallowed", func(t *testing.T) {
		_, err := resolveInteractiveOrganization(context.Background(),
			&fakeOrgAPI{err: errors.New("boom")}, &fakePrompter{}, "", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})

	t.Run("an aborted prompt is returned", func(t *testing.T) {
		orgs := []*action.MembershipItem{membership(orgAcme, true), membership(orgGlobex, false)}
		_, err := resolveInteractiveOrganization(context.Background(),
			&fakeOrgAPI{orgs: orgs}, &fakePrompter{selectErr: errAborted}, "", "")
		require.ErrorIs(t, err, errAborted)
	})
}

// --- project -----------------------------------------------------------------

func TestResolveInteractiveProject(t *testing.T) {
	testCases := []struct {
		name     string
		projects []string
		fromYML  string
		repoDir  string
		// readOnly are projects the caller can see but not create a workflow in
		readOnly []string
		// cannotCreate is the organization role refusing new projects
		cannotCreate bool
		// selectAnswer is what the user picks in the list; empty means the list
		// is not expected to be shown
		selectAnswer string
		// inputAnswer is what the user types when creating
		inputAnswer string
		wantValue   string
		wantSave    bool
		// wantOptions is the full option list, in order
		wantOptions []string
		// wantSelectDefault / wantInputDefault are the preselected values
		wantSelectDefault string
		wantInputDefault  string
		wantErr           string
	}{
		{
			name:             "with no visible projects the name is typed straight away",
			repoDir:          repoDirMyRepo,
			inputAnswer:      "my-repo",
			wantValue:        "my-repo",
			wantSave:         true,
			wantInputDefault: "my-repo",
		},
		{
			// Answering with what is already pinned writes nothing back, however
			// the answer was arrived at.
			name:             "with no visible projects the repository name is the default",
			fromYML:          "existing",
			repoDir:          repoDirMyRepo,
			inputAnswer:      "existing",
			wantValue:        "existing",
			wantSave:         false,
			wantInputDefault: "my-repo",
		},
		{
			name:              "the pinned project is preselected in the list",
			projects:          []string{projectAPI, "payments"},
			fromYML:           "payments",
			repoDir:           "whatever",
			selectAnswer:      "payments",
			wantValue:         "payments",
			wantSave:          false,
			wantOptions:       []string{projectAPI, "payments", createNewProjectOption},
			wantSelectDefault: "payments",
		},
		{
			name:              "with nothing pinned the first project is preselected",
			projects:          []string{projectAPI, "payments"},
			repoDir:           repoDirMyRepo,
			selectAnswer:      projectAPI,
			wantValue:         projectAPI,
			wantSave:          true,
			wantOptions:       []string{projectAPI, "payments", createNewProjectOption},
			wantSelectDefault: projectAPI,
		},
		{
			// The listing is the source of truth. Offering a name only
			// .chainloop.yml knows about makes it look like an existing project,
			// and picking it asks the control plane to create one, which fails
			// outright for a caller who may not create projects.
			name:              "a pinned project the organization does not have is not offered",
			projects:          []string{projectAPI},
			fromYML:           "gone-or-never-existed",
			repoDir:           "whatever",
			selectAnswer:      projectAPI,
			wantValue:         projectAPI,
			wantSave:          true,
			wantOptions:       []string{projectAPI, createNewProjectOption},
			wantSelectDefault: createNewProjectOption,
		},
		{
			// Nothing was ever created under that name, so it is as likely to be a
			// typo as an intention. The cursor starts on creating, since nothing
			// the repository points at can be picked, but the name is asked for
			// rather than proposed back.
			name:              "a pinned project the organization does not have does not seed the new name",
			projects:          []string{projectAPI},
			fromYML:           "gone-or-never-existed",
			repoDir:           repoDirMyRepo,
			selectAnswer:      createNewProjectOption,
			inputAnswer:       "gone-or-never-existed",
			wantValue:         "gone-or-never-existed",
			wantSave:          false,
			wantOptions:       []string{projectAPI, createNewProjectOption},
			wantSelectDefault: createNewProjectOption,
			wantInputDefault:  "my-repo",
		},
		{
			name:              "choosing to create opens an input prefilled with the repository name",
			projects:          []string{projectAPI},
			repoDir:           "My Cool Project",
			selectAnswer:      createNewProjectOption,
			inputAnswer:       "My Cool Project",
			wantValue:         "my-cool-project",
			wantSave:          true,
			wantOptions:       []string{projectAPI, createNewProjectOption},
			wantSelectDefault: projectAPI,
			wantInputDefault:  "my-cool-project",
		},
		{
			name:              "a free-form new name is normalized",
			projects:          []string{projectAPI},
			repoDir:           "repo",
			selectAnswer:      createNewProjectOption,
			inputAnswer:       "  Payments API (v2)!! ",
			wantValue:         "payments-api-v2",
			wantSave:          true,
			wantOptions:       []string{projectAPI, createNewProjectOption},
			wantSelectDefault: projectAPI,
			wantInputDefault:  "repo",
		},
		{
			name:         "a new name that normalizes to an existing project resolves to it",
			projects:     []string{projectAPI},
			repoDir:      "repo",
			selectAnswer: createNewProjectOption,
			inputAnswer:  "Backend API",
			wantValue:    projectAPI,
			wantSave:     true,
			wantOptions:  []string{projectAPI, createNewProjectOption},
		},
		{
			name:         "a name that normalizes to nothing is rejected",
			projects:     []string{projectAPI},
			repoDir:      "repo",
			selectAnswer: createNewProjectOption,
			inputAnswer:  nameWithNothingUsable,
			wantErr:      "cannot be empty",
		},
		{
			// Offering it would fail at creation time, which is the dead end
			// this whole prompt exists to remove.
			name:              "a project the caller can only view is not offered",
			projects:          []string{projectAPI},
			readOnly:          []string{"locked"},
			repoDir:           repoDirMyRepo,
			selectAnswer:      projectAPI,
			wantValue:         projectAPI,
			wantSave:          true,
			wantOptions:       []string{projectAPI, createNewProjectOption},
			wantSelectDefault: projectAPI,
		},
		{
			// The listing did carry it, so it is a real project, but picking it
			// would fail at creation time just the same.
			// It is a real project rather than a missing one, so the cursor stays
			// on the list instead of starting on creating it.
			name:              "a pinned project the caller can only view is neither offered nor preselected",
			projects:          []string{projectAPI},
			readOnly:          []string{"payments"},
			fromYML:           "payments",
			repoDir:           repoDirMyRepo,
			selectAnswer:      projectAPI,
			wantValue:         projectAPI,
			wantSave:          true,
			wantOptions:       []string{projectAPI, createNewProjectOption},
			wantSelectDefault: projectAPI,
		},
		{
			name:             "a pinned read-only project does not seed the new name",
			readOnly:         []string{"payments"},
			fromYML:          "payments",
			repoDir:          repoDirMyRepo,
			inputAnswer:      "my-repo",
			wantValue:        "my-repo",
			wantSave:         true,
			wantInputDefault: "my-repo",
		},
		{
			// Keeping a read-only project out of the list is not enough: typing
			// its name reaches the same dead end.
			name:        "typing the name of a project the caller can only view is refused",
			readOnly:    []string{"payments"},
			repoDir:     repoDirMyRepo,
			inputAnswer: "Payments",
			wantErr:     "you can only view the project",
		},
		{
			name:         "typing a read-only name is refused from the create entry too",
			projects:     []string{projectAPI},
			readOnly:     []string{"payments"},
			repoDir:      repoDirMyRepo,
			selectAnswer: createNewProjectOption,
			inputAnswer:  "payments",
			wantErr:      "you can only view the project",
		},
		{
			// Creating is an organization-role permission, so a contributor that
			// administers projects still cannot have the option: it would fail on
			// whatever name they typed.
			name:              "a caller who cannot create projects is not offered the option",
			projects:          []string{projectAPI, "payments"},
			cannotCreate:      true,
			repoDir:           repoDirMyRepo,
			selectAnswer:      projectAPI,
			wantValue:         projectAPI,
			wantSave:          true,
			wantOptions:       []string{projectAPI, "payments"},
			wantSelectDefault: projectAPI,
		},
		{
			name:              "a pinned project is still preselected without the create option",
			projects:          []string{projectAPI, "payments"},
			cannotCreate:      true,
			fromYML:           "payments",
			repoDir:           repoDirMyRepo,
			selectAnswer:      "payments",
			wantValue:         "payments",
			wantSave:          false,
			wantOptions:       []string{projectAPI, "payments"},
			wantSelectDefault: "payments",
		},
		{
			// Nothing to pick and nothing to create is a dead end, so it says so
			// instead of asking for a name the control plane would refuse.
			name:         "no writable project and no permission to create says so",
			readOnly:     []string{"payments"},
			cannotCreate: true,
			repoDir:      repoDirMyRepo,
			wantErr:      "you cannot create projects in this organization",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := &fakePrompter{selectAnswer: tc.selectAnswer, inputAnswer: tc.inputAnswer}
			got, err := resolveInteractiveProject(context.Background(),
				&fakeProjectLister{projects: tc.projects, readOnly: tc.readOnly, cannotCreate: tc.cannotCreate}, p, tc.fromYML, tc.repoDir, orgAcme)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantValue, got.value)
			assert.Equal(t, tc.wantSave, got.save)

			if tc.wantOptions == nil {
				assert.Empty(t, p.selects, "no list should have been shown")
			} else {
				require.Len(t, p.selects, 1)
				assert.Equal(t, tc.wantOptions, p.selects[0].options)
				if tc.wantSelectDefault != "" {
					assert.Equal(t, tc.wantSelectDefault, p.selects[0].defaultValue)
				}

				// The title offers creating a project exactly when the list does.
				wantTitle := selectProjectPromptTitle
				if slices.Contains(tc.wantOptions, createNewProjectOption) {
					wantTitle = projectPromptTitle
				}

				assert.Equal(t, wantTitle, p.selects[0].title)
				// Which organization the projects come from is part of the
				// question, not a line logged before it.
				assert.Equal(t, "organization  "+orgAcme, p.selects[0].description)
			}

			if tc.wantInputDefault != "" {
				require.Len(t, p.inputs, 1)
				assert.Equal(t, tc.wantInputDefault, p.inputs[0].defaultValue)
				assert.Equal(t, "organization  "+orgAcme, p.inputs[0].description,
					"a new project is created in it, so it is part of that question too")
			}
		})
	}
}

func TestResolveInteractiveProjectErrors(t *testing.T) {
	t.Run("a listing failure is returned, not swallowed", func(t *testing.T) {
		_, err := resolveInteractiveProject(context.Background(),
			&fakeProjectLister{err: errors.New("boom")}, &fakePrompter{}, "", "repo", orgAcme)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})

	t.Run("an aborted list is returned", func(t *testing.T) {
		_, err := resolveInteractiveProject(context.Background(),
			&fakeProjectLister{projects: []string{"a"}}, &fakePrompter{selectErr: errAborted}, "", "repo", orgAcme)
		require.ErrorIs(t, err, errAborted)
	})

	t.Run("an aborted input is returned", func(t *testing.T) {
		_, err := resolveInteractiveProject(context.Background(),
			&fakeProjectLister{}, &fakePrompter{inputErr: errAborted}, "", "repo", orgAcme)
		require.ErrorIs(t, err, errAborted)
	})
}

// --- non-interactive ---------------------------------------------------------

// TestResolveTraceIdentityNonInteractive covers the path a CI run takes. Test
// binaries have no terminal attached, so isInteractive is false here and the
// step never reaches the control plane.
func TestResolveTraceIdentityNonInteractive(t *testing.T) {
	t.Run("a missing project is reported, naming both ways to supply one", func(t *testing.T) {
		_, err := resolveTraceIdentity(context.Background(), &traceInitConfig{}, t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--project is required in non-interactive mode")
		assert.Contains(t, err.Error(), "projectName to .chainloop.yml")
	})
}

// --- aborting ----------------------------------------------------------------

// TestStopIfAborted pins down that dismissing a prompt ends the command
// quietly. Returning the error instead would exit 1 and print "ERROR aborted",
// which reads as a failure when the user simply changed their mind.
func TestStopIfAborted(t *testing.T) {
	other := errors.New("boom")

	testCases := []struct {
		name string
		err  error
		want error
	}{
		{name: "a dismissed prompt is not an error", err: errAborted, want: nil},
		{name: "a wrapped dismissal is recognized", err: fmt.Errorf("asking: %w", errAborted), want: nil},
		{name: "any other error is passed through", err: other, want: other},
		{name: "no error stays no error", err: nil, want: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, stopIfAborted(tc.err))
		})
	}
}
