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

package data

import (
	"fmt"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/project"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderProjectPredicate returns the SQL the predicate contributes to a query together with its
// bind arguments, so the tests can assert on the shape of the filter and on the values it carries,
// not only on the rows it would match. Values are parameterised, so they never appear in the SQL.
func renderProjectPredicate(t *testing.T, p func(*sql.Selector)) (string, []string) {
	t.Helper()
	sel := sql.Dialect(dialect.Postgres).Select("id").From(sql.Table(project.Table))
	p(sel)
	q, args := sel.Query()
	bound := make([]string, 0, len(args))
	for _, a := range args {
		bound = append(bound, fmt.Sprintf("%v", a))
	}
	return q, bound
}

func TestProjectVisibilityPredicate(t *testing.T) {
	orgs := make([]uuid.UUID, 0, 23)
	for i := 0; i < 23; i++ {
		orgs = append(orgs, uuid.New())
	}
	projA, projB := uuid.New(), uuid.New()

	testCases := []struct {
		name string
		// input
		orgIDs   []uuid.UUID
		rbac     map[uuid.UUID][]uuid.UUID
		wantNil  bool
		wantOrgs int // number of orgs expected in the organization IN list
		// the filter must stay a constant size however many orgs the caller belongs to
		wantExistsSubqueries int
		wantPairs            bool // the restricted branch matches (org, project) pairs
	}{
		{
			name:                 "no restrictions anywhere: a single organization IN list, no subquery",
			orgIDs:               orgs,
			rbac:                 map[uuid.UUID][]uuid.UUID{},
			wantOrgs:             23,
			wantExistsSubqueries: 0,
		},
		{
			name:                 "single organization behaves the same",
			orgIDs:               orgs[:1],
			rbac:                 map[uuid.UUID][]uuid.UUID{},
			wantOrgs:             1,
			wantExistsSubqueries: 0,
		},
		{
			name:                 "restricted everywhere: one pair list for every grant",
			orgIDs:               orgs[:2],
			rbac:                 map[uuid.UUID][]uuid.UUID{orgs[0]: {projA}, orgs[1]: {projB}},
			wantExistsSubqueries: 0,
			wantPairs:            true,
		},
		{
			name:                 "mixed: one organization list and one pair list",
			orgIDs:               orgs[:3],
			rbac:                 map[uuid.UUID][]uuid.UUID{orgs[0]: {projA}},
			wantOrgs:             2,
			wantExistsSubqueries: 0,
			wantPairs:            true,
		},
		{
			name:    "RBAC applies but nothing is visible: no predicate at all",
			orgIDs:  orgs[:2],
			rbac:    map[uuid.UUID][]uuid.UUID{orgs[0]: {}, orgs[1]: {}},
			wantNil: true,
		},
		{
			name:    "no organizations at all",
			orgIDs:  nil,
			rbac:    map[uuid.UUID][]uuid.UUID{},
			wantNil: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := projectVisibilityPredicate(tc.orgIDs, tc.rbac)
			if tc.wantNil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)

			query, _ := renderProjectPredicate(t, got)
			// A condition per organization is the regression this guards against.
			assert.Equal(t, tc.wantExistsSubqueries, strings.Count(query, "EXISTS"),
				"the filter must not grow a subquery per organization: %s", query)
			if tc.wantOrgs > 0 {
				assert.Contains(t, query, `"projects"."organization_id" IN (`,
					"organizations must be matched with a single IN list: %s", query)
			}
			if tc.wantPairs {
				assert.Contains(t, query, `("projects"."organization_id", "projects"."id") IN (`,
					"granted projects must be matched as (org, project) pairs: %s", query)
			}
		})
	}
}

// TestProjectVisibilityPredicateDoesNotCrossOrgs pins the property the pair matching exists for:
// a project granted to the caller in one org must not become visible because it sits in a
// different org the caller is also restricted in. Matching the org set and the project set
// independently would admit exactly that.
func TestProjectVisibilityPredicateDoesNotCrossOrgs(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	grantedInA, grantedInB := uuid.New(), uuid.New()

	got := projectVisibilityPredicate(
		[]uuid.UUID{orgA, orgB},
		map[uuid.UUID][]uuid.UUID{orgA: {grantedInA}, orgB: {grantedInB}},
	)
	require.NotNil(t, got)
	query, args := renderProjectPredicate(t, got)

	assert.Contains(t, query, `("projects"."organization_id", "projects"."id") IN (`,
		"the grants are matched as pairs, not as two independent sets: %s", query)
	// Each project is bound next to the org it was granted in, and to no other.
	assert.Equal(t, []string{orgA.String(), grantedInA.String(), orgB.String(), grantedInB.String()}, args,
		"every grant must keep its own organization")
	assert.NotContains(t, query, `"projects"."id" IN (`,
		"a bare project list would match a granted project in any of the caller's restricted orgs: %s", query)
}

// TestProjectVisibilityPredicateSkipsOrgsWithoutGrants keeps the behaviour that a restricted org
// with nothing granted contributes nothing at all, rather than widening either branch.
func TestProjectVisibilityPredicateSkipsOrgsWithoutGrants(t *testing.T) {
	orgFull, orgEmpty, orgGranted := uuid.New(), uuid.New(), uuid.New()
	granted := uuid.New()

	got := projectVisibilityPredicate(
		[]uuid.UUID{orgFull, orgEmpty, orgGranted},
		map[uuid.UUID][]uuid.UUID{orgEmpty: {}, orgGranted: {granted}},
	)
	require.NotNil(t, got)
	query, args := renderProjectPredicate(t, got)

	assert.Contains(t, args, orgFull.String(), "the unrestricted org is matched")
	assert.Contains(t, args, orgGranted.String(), "the org holding a grant is matched")
	assert.NotContains(t, args, orgEmpty.String(), "an org with no grant is never named: %s", query)
}
