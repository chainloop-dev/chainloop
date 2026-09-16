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

package data_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/predicate"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/referrer"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

// The visibility filter is written by hand because Ent's generated HasWorkflowsWith emits an
// uncorrelated IN. Hand-written SQL standing in for generated code needs more than a shape
// assertion, so these tests run both forms against the same data and require identical results
// for every state the filter can meet.
func TestReferrerVisibilityIntegration(t *testing.T) {
	suite.Run(t, new(referrerVisibilityIntegrationTestSuite))
}

type referrerVisibilityIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite

	// organizations
	orgAllowed, orgAlsoAllowed, orgOther uuid.UUID
	// referrers, named after the situation they are in
	noWorkflow              uuid.UUID
	liveInAllowed           uuid.UUID
	liveInOther             uuid.UUID
	deletedInAllowed        uuid.UUID
	deletedAllowedLiveOther uuid.UUID
	deletedAndLiveInAllowed uuid.UUID
	twoLiveInAllowed        uuid.UUID
	liveInTwoAllowedOrgs    uuid.UUID
}

// generatedVisibilityPredicate is the Ent-generated filter the repository replaced.
func generatedVisibilityPredicate(orgIDs []uuid.UUID) predicate.Referrer {
	return referrer.HasWorkflowsWith(
		workflow.DeletedAtIsNil(),
		workflow.HasOrganizationWith(organization.IDIn(orgIDs...)),
	)
}

// SetupTest builds every combination of organization and workflow state the filter can encounter:
// no workflow, a live one, a deleted one, and mixtures across organizations.
func (s *referrerVisibilityIntegrationTestSuite) SetupTest() {
	s.UseCasesEachTestSuite.SetupTest()
	ctx := context.Background()
	client := s.Data.DB

	org := func(name string) uuid.UUID {
		return client.Organization.Create().SetName(name).SaveX(ctx).ID
	}
	s.orgAllowed, s.orgAlsoAllowed, s.orgOther = org("allowed"), org("also-allowed"), org("other")

	newWorkflow := func(name string, orgID uuid.UUID, deleted bool) uuid.UUID {
		contract := client.WorkflowContract.Create().SetName(name + "-contract").SetOrganizationID(orgID).SaveX(ctx)
		project := client.Project.Create().SetName(name + "-project").SetOrganizationID(orgID).SaveX(ctx)
		b := client.Workflow.Create().SetName(name).SetOrganizationID(orgID).
			SetProjectID(project.ID).SetContractID(contract.ID)
		if deleted {
			b = b.SetDeletedAt(time.Now())
		}
		return b.SaveX(ctx).ID
	}

	liveAllowed := newWorkflow("live-allowed", s.orgAllowed, false)
	liveAllowed2 := newWorkflow("live-allowed-2", s.orgAllowed, false)
	liveAlsoAllowed := newWorkflow("live-also-allowed", s.orgAlsoAllowed, false)
	liveOther := newWorkflow("live-other", s.orgOther, false)
	deletedAllowed := newWorkflow("deleted-allowed", s.orgAllowed, true)

	n := 0
	newReferrer := func(kind string, workflowIDs ...uuid.UUID) uuid.UUID {
		n++
		return client.Referrer.Create().
			SetDigest(fmt.Sprintf("sha256:%064d", n)).
			SetKind(kind).
			SetDownloadable(true).
			AddWorkflowIDs(workflowIDs...).
			SaveX(ctx).ID
	}

	s.noWorkflow = newReferrer("ATTESTATION")
	s.liveInAllowed = newReferrer("ATTESTATION", liveAllowed)
	s.liveInOther = newReferrer("ATTESTATION", liveOther)
	s.deletedInAllowed = newReferrer("ATTESTATION", deletedAllowed)
	s.deletedAllowedLiveOther = newReferrer("ATTESTATION", deletedAllowed, liveOther)
	s.deletedAndLiveInAllowed = newReferrer("ATTESTATION", deletedAllowed, liveAllowed)
	s.twoLiveInAllowed = newReferrer("SBOM_CYCLONEDX_JSON", liveAllowed, liveAllowed2)
	s.liveInTwoAllowedOrgs = newReferrer("CONTAINER_IMAGE", liveAllowed, liveAlsoAllowed)

	// Give the root references so the traversal case has something to return.
	client.Referrer.UpdateOneID(s.liveInAllowed).
		AddReferenceIDs(s.twoLiveInAllowed, s.liveInOther, s.deletedInAllowed).ExecX(ctx)
}

func (s *referrerVisibilityIntegrationTestSuite) TestMatchesTheGeneratedPredicate() {
	ctx := context.Background()

	// Expectations are written out rather than derived, so a change in either form has to be
	// justified here.
	testCases := []struct {
		name string
		orgs []uuid.UUID
		want []uuid.UUID
	}{
		{
			name: "one allowed org: live workflows in it, nothing else",
			orgs: []uuid.UUID{s.orgAllowed},
			want: []uuid.UUID{s.liveInAllowed, s.deletedAndLiveInAllowed, s.twoLiveInAllowed, s.liveInTwoAllowedOrgs},
		},
		{
			name: "the other org only",
			orgs: []uuid.UUID{s.orgOther},
			want: []uuid.UUID{s.liveInOther, s.deletedAllowedLiveOther},
		},
		{
			name: "two allowed orgs: the union, each referrer once",
			orgs: []uuid.UUID{s.orgAllowed, s.orgAlsoAllowed},
			want: []uuid.UUID{s.liveInAllowed, s.deletedAndLiveInAllowed, s.twoLiveInAllowed, s.liveInTwoAllowedOrgs},
		},
		{
			name: "all three orgs: everything with a live workflow",
			orgs: []uuid.UUID{s.orgAllowed, s.orgAlsoAllowed, s.orgOther},
			want: []uuid.UUID{
				s.liveInAllowed, s.liveInOther, s.deletedAllowedLiveOther,
				s.deletedAndLiveInAllowed, s.twoLiveInAllowed, s.liveInTwoAllowedOrgs,
			},
		},
		{
			name: "duplicated org ids must not duplicate results",
			orgs: []uuid.UUID{s.orgAllowed, s.orgAllowed, s.orgAllowed},
			want: []uuid.UUID{s.liveInAllowed, s.deletedAndLiveInAllowed, s.twoLiveInAllowed, s.liveInTwoAllowedOrgs},
		},
		{
			name: "an org with no workflows at all",
			orgs: []uuid.UUID{uuid.New()},
			want: nil,
		},
		{
			name: "no orgs at all",
			orgs: nil,
			want: nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			generated := s.referrerIDs(ctx, generatedVisibilityPredicate(tc.orgs))
			handWritten := s.referrerIDs(ctx, data.ReferrerVisibleToOrgsForTest(tc.orgs))

			s.Equal(sortIDs(tc.want), generated, "the generated filter no longer matches the documented expectation")
			s.Equal(generated, handWritten, "the hand-written filter must select exactly what the generated one does")
		})
	}

	// A referrer is never returned twice, which is what makes dropping Ent's DISTINCT safe: the
	// filter is an EXISTS, so it cannot multiply rows the way a join would.
	s.Run("a referrer in several matching workflows is returned once, without DISTINCT", func() {
		orgs := []uuid.UUID{s.orgAllowed, s.orgAlsoAllowed, s.orgOther}
		ids, err := s.Data.DB.Referrer.Query().
			Where(data.ReferrerVisibleToOrgsForTest(orgs)).
			Unique(false).
			IDs(ctx)
		s.Require().NoError(err)

		counts := map[uuid.UUID]int{}
		for _, id := range ids {
			counts[id]++
		}
		s.Equal(1, counts[s.twoLiveInAllowed], "a referrer in two workflows of one org appears once")
		s.Equal(1, counts[s.liveInTwoAllowedOrgs], "a referrer in workflows of two orgs appears once")
	})

	// The referrer queries combine visibility with digest and kind filters, so the hand-written
	// selector has to behave when it is not the only predicate.
	s.Run("composes with other predicates", func() {
		orgs := []uuid.UUID{s.orgAllowed}
		generated := s.referrerIDs(ctx, generatedVisibilityPredicate(orgs), referrer.KindEQ("ATTESTATION"))
		handWritten := s.referrerIDs(ctx, data.ReferrerVisibleToOrgsForTest(orgs), referrer.KindEQ("ATTESTATION"))
		s.Equal(generated, handWritten)
		s.NotEmpty(handWritten, "the fixture must exercise this, not pass by returning nothing")
	})

	// doGet applies the filter to root.QueryReferences(), a different query shape where the
	// selector is already joined through the references table.
	s.Run("behaves the same when traversing references", func() {
		orgs := []uuid.UUID{s.orgAllowed, s.orgOther}
		root := s.Data.DB.Referrer.GetX(ctx, s.liveInAllowed)

		generated, err := root.QueryReferences().Where(generatedVisibilityPredicate(orgs)).IDs(ctx)
		s.Require().NoError(err)
		handWritten, err := root.QueryReferences().Where(data.ReferrerVisibleToOrgsForTest(orgs)).Unique(false).IDs(ctx)
		s.Require().NoError(err)

		s.Equal(sortIDs(generated), sortIDs(handWritten))
		s.NotEmpty(handWritten, "the fixture must exercise this, not pass by returning nothing")
	})
}

func (s *referrerVisibilityIntegrationTestSuite) referrerIDs(ctx context.Context, preds ...predicate.Referrer) []uuid.UUID {
	s.T().Helper()
	ids, err := s.Data.DB.Referrer.Query().Where(preds...).IDs(ctx)
	s.Require().NoError(err)
	return sortIDs(ids)
}

func sortIDs(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	out := append([]uuid.UUID(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}
