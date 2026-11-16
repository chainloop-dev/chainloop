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
	"slices"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/referrer"
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

	var wfA, wfB uuid.UUID
	s.orgA, wfA = newWorkflow()
	s.orgB, wfB = newWorkflow()
	s.materialDigest = fmt.Sprintf("sha256:%064d", 0)

	store := func(orgID, wfID uuid.UUID) {
		digest := fmt.Sprintf("sha256:%064d", len(s.attestations)+1)
		att := &biz.Referrer{Digest: digest, Kind: biz.ReferrerAttestationType, Downloadable: true}
		material := &biz.Referrer{Digest: s.materialDigest, Kind: sharedMaterialKind, Downloadable: true}
		// both directions, as the attestation extraction stores them
		att.References = []*biz.Referrer{{Digest: material.Digest, Kind: material.Kind}}
		material.References = []*biz.Referrer{{Digest: att.Digest, Kind: att.Kind}}
		s.Require().NoError(s.Repos.Referrer.Save(ctx, []*biz.Referrer{att, material}, wfID))
		s.attestations = append(s.attestations, storedAttestation{digest: digest, org: orgID})
	}

	// Interleave the organizations so that the newest-first order mixes them.
	for range 3 {
		store(s.orgA, wfA)
		store(s.orgB, wfB)
	}
	store(s.orgA, wfA)
	store(s.orgA, wfA)
}

func (s *referrerReferencesIntegrationTestSuite) TestReferenceStoresTheReferencedCreationTime() {
	ctx := context.Background()
	client := s.Data.DB

	createdAt := make(map[uuid.UUID]int64)
	for _, r := range client.Referrer.Query().AllX(ctx) {
		createdAt[r.ID] = r.CreatedAt.UnixMicro()
	}

	refs := client.ReferrerReference.Query().AllX(ctx)
	// each attestation has a reference to the material and one back
	s.Require().Len(refs, 2*len(s.attestations))
	for _, ref := range refs {
		// The material was stored with the first attestation. Every later reference to it must
		// still carry the material's own creation time, not the time the reference was stored.
		s.Equal(createdAt[ref.ReferredByID], ref.CreatedAt.UnixMicro(),
			"reference %s -> %s", ref.ReferrerID, ref.ReferredByID)
	}
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
	s.T().Helper()
	ctx := context.Background()

	var digests []string
	cursor, pages := "", 0
	for pages < 20 {
		p, err := pagination.NewCursor(cursor, limit)
		s.Require().NoError(err)
		got, next, err := s.Repos.Referrer.GetFromRoot(ctx, s.materialDigest, orgs, p)
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
