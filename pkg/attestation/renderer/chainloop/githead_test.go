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

package chainloop

import (
	"testing"

	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestGitHeadVerifications(t *testing.T) {
	const (
		verified = "verified"
		sha1     = "sha1"
		pgp      = "PGP"
	)

	annotations := func(t *testing.T, fields map[string]any) *structpb.Struct {
		t.Helper()
		s, err := structpb.NewStruct(fields)
		require.NoError(t, err)
		return s
	}

	statement := &intoto.Statement{Subject: []*intoto.ResourceDescriptor{
		{Name: "workflow.wf"},
		{Name: SubjectGitHead, Digest: map[string]string{sha1: "c1"}, Annotations: annotations(t, map[string]any{subjectGitAnnotationAuthorEmail: "a@b"})},
		{Name: SubjectGitHead, Digest: map[string]string{sha1: "c2"}, Annotations: annotations(t, map[string]any{subjectGitAnnotationAuthorVerificationStatus: verified})},
		{Name: SubjectGitHead, Digest: map[string]string{sha1: "c3"}, Annotations: annotations(t, map[string]any{
			subjectGitAnnotationAuthorVerificationStatus: verified, subjectGitAnnotationSignatureAlgorithm: pgp,
		})},
	}}

	got := GitHeadVerifications(statement)
	assert.Equal(t, []GitHeadVerification{{Commit: "c2", Status: verified}, {Commit: "c3", Status: verified, SignatureAlgorithm: pgp}}, got)
	assert.Empty(t, GitHeadVerifications(&intoto.Statement{}))
	assert.Empty(t, GitHeadVerifications(nil))
}

func TestPredicateRunner(t *testing.T) {
	p := &ProvenancePredicateV02{ProvenancePredicateCommon: &ProvenancePredicateCommon{RunnerType: "CHAINLOOP_SANDBOX", RunnerAuthenticated: true}}

	var normalizable NormalizablePredicate = p
	assert.Equal(t, "CHAINLOOP_SANDBOX", normalizable.GetRunnerType())
	assert.True(t, normalizable.GetRunnerAuthenticated())
}
