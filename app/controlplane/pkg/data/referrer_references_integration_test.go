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
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/referrer"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/referrerreference"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

// A material shared by many attestations, such as an identical scanner report, collects one
// reference per attestation in every organization. Discovery pages over those references newest
// first, so each page must be read in order from the references of the root instead of sorting
// all of them.
func TestReferrerReferencesIntegration(t *testing.T) {
	suite.Run(t, new(referrerReferencesIntegrationTestSuite))
}

type referrerReferencesIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite

	orgA, orgB uuid.UUID
	wfA, wfB   uuid.UUID
	// the material every attestation references
	materialDigest string
	// the attestations in storage order, oldest first
	attestations []storedAttestation
}

type storedAttestation struct {
	digest string
	org    uuid.UUID
}

// newestFirst returns the digests of the attestations stored in the given organizations, newest
// first.
func (s *referrerReferencesIntegrationTestSuite) newestFirst(orgs ...uuid.UUID) []string {
	var digests []string
	for _, att := range s.attestations {
		if slices.Contains(orgs, att.org) {
			digests = append(digests, att.digest)
		}
	}
	slices.Reverse(digests)
	return digests
}

const sharedMaterialKind = "GITLEAKS_JSON"

func (s *referrerReferencesIntegrationTestSuite) SetupTest() {
	s.UseCasesEachTestSuite.SetupTest()
	ctx := context.Background()
	s.attestations = nil

	newWorkflow := func() (uuid.UUID, uuid.UUID) {
		org, err := s.Organization.CreateWithRandomName(ctx)
		s.Require().NoError(err)
		wf, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "wf", OrgID: org.ID, Project: "project"})
		s.Require().NoError(err)
		return uuid.MustParse(org.ID), wf.ID
	}

	s.orgA, s.wfA = newWorkflow()
	s.orgB, s.wfB = newWorkflow()
	s.materialDigest = fmt.Sprintf("sha256:%064d", 0)

	store := func(orgID, wfID uuid.UUID) {
		digest := fmt.Sprintf("sha256:%064d", len(s.attestations)+1)
		s.saveAttestation(digest, wfID)
		s.attestations = append(s.attestations, storedAttestation{digest: digest, org: orgID})
	}

	// Interleave the organizations so that the newest-first order mixes them.
	for range 3 {
		store(s.orgA, s.wfA)
		store(s.orgB, s.wfB)
	}
	store(s.orgA, s.wfA)
	store(s.orgA, s.wfA)
}

// saveAttestation stores an attestation of the shared material in the given workflow, as the
// attestation extraction does, with any other attestations as materials.
func (s *referrerReferencesIntegrationTestSuite) saveAttestation(digest string, wfID uuid.UUID, attestationMaterials ...string) {
	s.T().Helper()
	att := &biz.Referrer{Digest: digest, Kind: biz.ReferrerAttestationType, Downloadable: true}
	material := &biz.Referrer{Digest: s.materialDigest, Kind: sharedMaterialKind, Downloadable: true}
	// both directions, as the attestation extraction stores them
	att.References = []*biz.Referrer{{Digest: material.Digest, Kind: material.Kind}}
	material.References = []*biz.Referrer{{Digest: att.Digest, Kind: att.Kind}}
	referrers := make([]*biz.Referrer, 0, 2+len(attestationMaterials))
	referrers = append(referrers, att, material)
	for _, d := range attestationMaterials {
		att.References = append(att.References, &biz.Referrer{Digest: d, Kind: biz.ReferrerAttestationType})
		referrers = append(referrers, &biz.Referrer{
			Digest: d, Kind: biz.ReferrerAttestationType, Downloadable: true,
			References: []*biz.Referrer{{Digest: att.Digest, Kind: att.Kind}},
		})
	}
	s.Require().NoError(s.Repos.Referrer.Save(context.Background(), referrers, wfID))
}

// idOf returns the ID of the referrer with the given digest.
func (s *referrerReferencesIntegrationTestSuite) idOf(digest string) uuid.UUID {
	s.T().Helper()
	return s.Data.DB.Referrer.Query().Where(referrer.Digest(digest)).OnlyIDX(context.Background())
}

func (s *referrerReferencesIntegrationTestSuite) TestReferenceStoredColumns() {
	ctx := context.Background()
	client := s.Data.DB

	createdAt := make(map[uuid.UUID]int64)
	for _, r := range client.Referrer.Query().AllX(ctx) {
		createdAt[r.ID] = r.CreatedAt.UnixMicro()
	}
	orgOf := make(map[uuid.UUID]uuid.UUID)
	for _, att := range s.attestations {
		orgOf[s.idOf(att.digest)] = att.org
	}

	refs := client.ReferrerReference.Query().AllX(ctx)
	// each attestation has a reference to the material and one back
	s.Require().Len(refs, 2*len(s.attestations))
	for _, ref := range refs {
		// The material was stored with the first attestation. Every later reference to it must
		// still carry the material's own creation time, not the time the reference was stored.
		s.Equal(createdAt[ref.ReferredByID], ref.CreatedAt.UnixMicro(),
			"reference %s -> %s", ref.ReferrerID, ref.ReferredByID)

		// one end of each reference is the attestation that stored it
		org, ok := orgOf[ref.ReferrerID]
		if !ok {
			org = orgOf[ref.ReferredByID]
		}
		s.Require().NotNil(ref.OrganizationID, "reference %s -> %s", ref.ReferrerID, ref.ReferredByID)
		s.Equal(org, *ref.OrganizationID, "reference %s -> %s", ref.ReferrerID, ref.ReferredByID)
	}
}

// The same attestation can be stored again in a run of another organization. Its references then
// belong to both organizations: they lose their organization and are read in both.
func (s *referrerReferencesIntegrationTestSuite) TestReferenceStoredFromTwoOrganizations() {
	ctx := context.Background()
	client := s.Data.DB

	first := s.attestations[0]
	s.Require().Equal(s.orgA, first.org)
	s.saveAttestation(first.digest, s.wfB)

	firstID := s.idOf(first.digest)
	for _, ref := range client.ReferrerReference.Query().AllX(ctx) {
		if ref.ReferrerID == firstID || ref.ReferredByID == firstID {
			s.Nil(ref.OrganizationID, "reference %s -> %s", ref.ReferrerID, ref.ReferredByID)
		} else {
			s.NotNil(ref.OrganizationID, "reference %s -> %s", ref.ReferrerID, ref.ReferredByID)
		}
	}

	// Storing it again in the same organization leaves the references as they are
	s.saveAttestation(s.attestations[1].digest, s.wfB)
	secondID := s.idOf(s.attestations[1].digest)
	for _, ref := range client.ReferrerReference.Query().AllX(ctx) {
		if ref.ReferrerID == secondID || ref.ReferredByID == secondID {
			s.Require().NotNil(ref.OrganizationID)
			s.Equal(s.orgB, *ref.OrganizationID)
		}
	}

	// The first attestation is now read in both organizations, in its place in the order
	got, _ := s.collectPages([]uuid.UUID{s.orgB}, 2)
	s.Equal(append(s.newestFirst(s.orgB), first.digest), got)
	got, _ = s.collectPages([]uuid.UUID{s.orgA}, 2)
	s.Equal(s.newestFirst(s.orgA), got)
	got, _ = s.collectPages([]uuid.UUID{s.orgA, s.orgB}, 3)
	s.Equal(s.newestFirst(s.orgA, s.orgB), got)
}

// An attestation can be a material of an attestation in another organization. That links it to
// the other organization too, but the references that its own organization stored stay there:
// the other organization reads it only through the references that its own attestation stored.
func (s *referrerReferencesIntegrationTestSuite) TestAttestationMaterialFromAnotherOrganization() {
	fromA := s.attestations[0]
	s.Require().Equal(s.orgA, fromA.org)

	inB := fmt.Sprintf("sha256:%064d", 100)
	s.saveAttestation(inB, s.wfB, fromA.digest)

	// Through the shared material, organization B reads its own attestations only
	got, _ := s.collectPages([]uuid.UUID{s.orgB}, 20)
	s.Equal(append([]string{inB}, s.newestFirst(s.orgB)...), got)

	// Through its own attestation, organization B reads the attestation of organization A
	got, _ = s.collectPagesFrom(inB, []uuid.UUID{s.orgB}, 20)
	s.ElementsMatch([]string{s.materialDigest, fromA.digest}, got)
}

// The migration fills in the organization of the existing references from the organizations both
// ends belong to, since nothing recorded which attestation stored them. It must give what storing
// them gives, or NULL where it cannot tell: NULL is read in every organization, so it never hides
// a reference.
func (s *referrerReferencesIntegrationTestSuite) TestBackfillMatchesTheStoredOrganization() {
	ctx := context.Background()
	client := s.Data.DB

	// Add the cases that link a referrer to more than one organization
	repushed := s.attestations[0]
	s.saveAttestation(repushed.digest, s.wfB)
	fromA := s.attestations[2]
	s.Require().Equal(s.orgA, fromA.org)
	s.saveAttestation(fmt.Sprintf("sha256:%064d", 100), s.wfB, fromA.digest)

	type key struct{ from, to uuid.UUID }
	type value struct {
		org       *uuid.UUID
		createdAt int64
	}
	snapshot := func() map[key]value {
		out := make(map[key]value)
		for _, ref := range client.ReferrerReference.Query().AllX(ctx) {
			out[key{ref.ReferrerID, ref.ReferredByID}] = value{ref.OrganizationID, ref.CreatedAt.UnixMicro()}
		}
		return out
	}
	stored := snapshot()

	_, err := s.Data.SQLDB.ExecContext(ctx, `UPDATE referrer_references SET organization_id = NULL, created_at = $1`, time.Now())
	s.Require().NoError(err)
	_, err = s.Data.SQLDB.ExecContext(ctx, s.backfillStatement())
	s.Require().NoError(err)
	backfilled := snapshot()

	materialID := s.idOf(s.materialDigest)
	fromAID := s.idOf(fromA.digest)
	s.Require().Len(backfilled, len(stored))
	for k, want := range stored {
		got := backfilled[k]
		s.Equal(want.createdAt, got.createdAt, "reference %s -> %s", k.from, k.to)
		if (k == key{materialID, fromAID}) || (k == key{fromAID, materialID}) {
			// Organization A stored these, but both ends are now in both organizations, so the
			// migration cannot tell which one did.
			s.Require().NotNil(want.org)
			s.Nil(got.org, "reference %s -> %s", k.from, k.to)
			continue
		}
		s.Equal(want.org, got.org, "reference %s -> %s", k.from, k.to)
	}
}

// backfillStatement returns the UPDATE of the migration that fills in the reference columns.
func (s *referrerReferencesIntegrationTestSuite) backfillStatement() string {
	s.T().Helper()
	migration, err := os.ReadFile("ent/migrate/migrations/20261116124543.sql")
	s.Require().NoError(err)
	for _, statement := range strings.Split(string(migration), ";\n") {
		if strings.Contains(statement, "UPDATE \"referrer_references\"") {
			return statement
		}
	}
	s.Require().Fail("the migration has no backfill statement")
	return ""
}

// The organization of a reference only narrows what a page reads. A reference with no
// organization, or with the caller's organization, is still returned only when the caller can see
// the referrer it points to.
func (s *referrerReferencesIntegrationTestSuite) TestOrganizationNeverGrantsVisibility() {
	ctx := context.Background()
	client := s.Data.DB

	// Two attestations visible in organization B only, referenced from the shared material
	onlyInB := func(n int) *ent.Referrer {
		return client.Referrer.Create().SetDigest(fmt.Sprintf("sha256:%064d", n)).
			SetKind(biz.ReferrerAttestationType).SetDownloadable(true).AddWorkflowIDs(s.wfB).SaveX(ctx)
	}
	withNoOrg, withOrgA := onlyInB(200), onlyInB(201)
	materialID := s.idOf(s.materialDigest)
	client.Referrer.UpdateOneID(materialID).AddReferenceIDs(withNoOrg.ID, withOrgA.ID).ExecX(ctx)
	// One reference has no organization, the other claims organization A
	client.ReferrerReference.Update().
		Where(referrerreference.ReferrerID(materialID), referrerreference.ReferredByID(withOrgA.ID)).
		SetOrganizationID(s.orgA).ExecX(ctx)

	got, _ := s.collectPages([]uuid.UUID{s.orgA}, 2)
	s.Equal(s.newestFirst(s.orgA), got, "organization A must not see referrers of organization B")

	got, _ = s.collectPages([]uuid.UUID{s.orgB}, 2)
	s.Contains(got, withNoOrg.Digest, "a reference with no organization is read in organization B")
	s.NotContains(got, withOrgA.Digest, "a reference stored in organization A is not read in organization B")
}

func (s *referrerReferencesIntegrationTestSuite) TestPagesNewestFirst() {
	testCases := []struct {
		name  string
		orgs  []uuid.UUID
		limit int
	}{
		{name: "one organization, pages of two", orgs: []uuid.UUID{s.orgA}, limit: 2},
		{name: "one organization, one page", orgs: []uuid.UUID{s.orgA}, limit: 20},
		{name: "other organization, pages of one", orgs: []uuid.UUID{s.orgB}, limit: 1},
		{name: "both organizations, pages of three", orgs: []uuid.UUID{s.orgA, s.orgB}, limit: 3},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			want := s.newestFirst(tc.orgs...)
			got, pages := s.collectPages(tc.orgs, tc.limit)
			s.Equal(want, got)
			s.Equal((len(want)+tc.limit-1)/tc.limit, pages)
		})
	}
}

// A reference stored without an explicit creation time, for example by a control plane that runs
// the previous release during a rolling upgrade, still takes part in the pages. It sorts by the
// time it was stored.
func (s *referrerReferencesIntegrationTestSuite) TestReferenceWithoutExplicitCreationTime() {
	ctx := context.Background()
	client := s.Data.DB

	material := client.Referrer.Query().Where(referrer.Digest(s.materialDigest)).OnlyX(ctx)
	wf := client.Workflow.Query().FirstIDX(ctx)
	late := client.Referrer.Create().SetDigest(fmt.Sprintf("sha256:%064d", 99)).
		SetKind(biz.ReferrerAttestationType).SetDownloadable(true).AddWorkflowIDs(wf).SaveX(ctx)
	client.Referrer.UpdateOneID(material.ID).AddReferenceIDs(late.ID).ExecX(ctx)

	orgs := []uuid.UUID{s.orgA, s.orgB}
	got, _ := s.collectPages(orgs, 2)
	s.Require().Len(got, len(s.attestations)+1)
	s.Equal(late.Digest, got[0], "the latest stored reference comes first")
}

// collectPages follows the cursor from the shared material and returns the attestation digests
// in the order they were returned, and the number of pages read.
func (s *referrerReferencesIntegrationTestSuite) collectPages(orgs []uuid.UUID, limit int) ([]string, int) {
	return s.collectPagesFrom(s.materialDigest, orgs, limit)
}

// collectPagesFrom follows the cursor from the given root.
func (s *referrerReferencesIntegrationTestSuite) collectPagesFrom(root string, orgs []uuid.UUID, limit int) ([]string, int) {
	s.T().Helper()
	ctx := context.Background()

	var digests []string
	cursor, pages := "", 0
	for pages < 20 {
		p, err := pagination.NewCursor(cursor, limit)
		s.Require().NoError(err)
		got, next, err := s.Repos.Referrer.GetFromRoot(ctx, root, orgs, p)
		s.Require().NoError(err)
		s.Require().NotNil(got)
		pages++
		for _, ref := range got.References {
			digests = append(digests, ref.Digest)
		}
		if next == "" {
			break
		}
		cursor = next
	}

	return digests, pages
}
