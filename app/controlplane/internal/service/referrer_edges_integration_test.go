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

package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// The RPC decides which of two authorization paths to take and maps referrers onto the indexes
// the caller gave. Neither is exercised by the use-case tests, so both are covered here through
// the service itself.
func TestReferrerEdgesServiceIntegration(t *testing.T) {
	suite.Run(t, new(referrerEdgesServiceSuite))
}

type referrerEdgesServiceSuite struct {
	testhelpers.UseCasesEachTestSuite

	svc         *service.ReferrerService
	org         *biz.Organization
	user        *biz.User
	workflow    *biz.Workflow
	apiToken    *biz.APIToken
	attestation *biz.ReferrerRef
	materials   []*biz.ReferrerRef
}

func (s *referrerEdgesServiceSuite) SetupTest() {
	ctx := context.Background()
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())
	s.svc = service.NewReferrerService(s.Referrer)

	var err error
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	require.NoError(s.T(), err)
	s.user, err = s.User.UpsertByEmail(ctx, "edges@chainloop.local", nil)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org.ID, s.user.ID, biz.WithCurrentMembership())
	require.NoError(s.T(), err)

	s.workflow, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{
		OrgID: s.org.ID, Name: "edges-wf", Project: "edges-project",
	})
	require.NoError(s.T(), err)

	s.apiToken, err = s.APIToken.Create(ctx, "edges-token", nil, nil, &s.org.ID)
	require.NoError(s.T(), err)

	envelope, raw := edgesEnvelope(s.T())
	digest, _, err := v1.SHA256(bytes.NewReader(raw))
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.Referrer.ExtractAndPersist(ctx, envelope, digest, s.workflow.ID.String()))

	root, _, err := s.Referrer.GetFromRootUser(ctx, digest.String(), "", s.user.ID, nil)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), root.References)

	s.attestation = &biz.ReferrerRef{Digest: root.Digest, Kind: root.Kind}
	for _, ref := range root.References {
		s.materials = append(s.materials, &biz.ReferrerRef{Digest: ref.Digest, Kind: ref.Kind})
	}
}

func (s *referrerEdgesServiceSuite) userCtx() context.Context {
	ctx := entities.WithCurrentUser(context.Background(), &entities.User{ID: s.user.ID, Email: s.user.Email})
	return entities.WithCurrentOrg(ctx, &entities.Org{ID: s.org.ID, Name: s.org.Name})
}

func (s *referrerEdgesServiceSuite) tokenCtx() context.Context {
	ctx := entities.WithCurrentOrg(context.Background(), &entities.Org{ID: s.org.ID, Name: s.org.Name})
	ctx = entities.WithCurrentAPIToken(ctx, &entities.APIToken{ID: s.apiToken.ID.String(), Name: s.apiToken.Name})
	return usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: s.apiToken.ID.String()}).String())
}

func (s *referrerEdgesServiceSuite) request(nodes ...*biz.ReferrerRef) *pb.ReferrerServiceDiscoverEdgesRequest {
	out := make([]*pb.ReferrerRef, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, &pb.ReferrerRef{Digest: n.Digest, Kind: n.Kind})
	}
	return &pb.ReferrerServiceDiscoverEdgesRequest{Nodes: out}
}

func (s *referrerEdgesServiceSuite) TestDiscoverEdges() {
	s.Run("a logged-in user gets one edge per material, indexed by request position", func() {
		nodes := append([]*biz.ReferrerRef{s.attestation}, s.materials...)
		res, err := s.svc.DiscoverEdges(s.userCtx(), s.request(nodes...))
		s.Require().NoError(err)
		s.Len(res.GetEdges(), len(s.materials))
		for _, e := range res.GetEdges() {
			s.Less(e.GetFrom(), e.GetTo(), "indexes come back normalised")
			s.EqualValues(0, e.GetFrom(), "the attestation is at index 0 of this request")
		}
	})

	s.Run("a referrer the caller cannot see contributes no edges", func() {
		unknown := &biz.ReferrerRef{Digest: "sha256:" + "00", Kind: "ATTESTATION"}
		res, err := s.svc.DiscoverEdges(s.userCtx(), s.request(s.attestation, unknown))
		s.Require().NoError(err)
		s.Empty(res.GetEdges(), "an unresolvable node must not fail the call")
	})

	s.Run("no credential is refused", func() {
		_, err := s.svc.DiscoverEdges(context.Background(), s.request(s.attestation, s.materials[0]))
		s.Error(err)
	})

	s.Run("an API token resolves through its own organization", func() {
		nodes := append([]*biz.ReferrerRef{s.attestation}, s.materials...)
		res, err := s.svc.DiscoverEdges(s.tokenCtx(), s.request(nodes...))
		s.Require().NoError(err)
		s.Len(res.GetEdges(), len(s.materials), "the token path must see what the user path sees")
	})

	// Project scoping is deliberately not covered here. It resolves through workflow runs, and
	// this fixture has none, so every project name would answer the same way and the assertion
	// would hold for the wrong reason. It belongs where a run fixture already exists.
}

func edgesEnvelope(t *testing.T) (*dsse.Envelope, []byte) {
	raw, err := os.ReadFile("../../pkg/biz/testdata/attestations/with-git-subject.json")
	require.NoError(t, err)
	var envelope *dsse.Envelope
	require.NoError(t, json.Unmarshal(raw, &envelope))

	return envelope, raw
}
