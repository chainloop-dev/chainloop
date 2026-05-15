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

package s3accesspoint

import (
	"bytes"
	"context"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackend_FailClosedWithoutRequestingOrg is the load-bearing fail-
// closed test: any backend operation that would normally hit AWS must
// refuse to even attempt the call when the caller forgot to enrich the
// context with WithRequestingOrg. This test does NOT need LocalStack —
// the credential provider rejects the request before any AWS SDK code
// runs.
//
// Not parallel: uses t.Setenv to fence the AWS SDK off from the real
// credential chain.
func TestBackend_FailClosedWithoutRequestingOrg(t *testing.T) {
	b := newTestBackend(t)
	ctx := context.Background() // intentionally no WithRequestingOrg

	t.Run("upload", func(t *testing.T) {
		err := b.Upload(ctx, bytes.NewReader([]byte("data")),
			&pb.CASResource{Digest: "deadbeef", FileName: "x.txt"})
		assertFailedClosed(t, err)
	})

	t.Run("download", func(t *testing.T) {
		// Download calls Exists -> Describe -> HeadObject, which goes
		// through the credentials provider and trips the fail-closed
		// path before any AWS call is made.
		err := b.Download(ctx, &bytes.Buffer{}, "deadbeef")
		assertFailedClosed(t, err)
	})

	t.Run("describe", func(t *testing.T) {
		_, err := b.Describe(ctx, "deadbeef")
		assertFailedClosed(t, err)
	})

	t.Run("check-write", func(t *testing.T) {
		// CheckWritePermissions has its own pre-flight assertion that
		// short-circuits without consulting the credentials provider at
		// all, which is both faster and gives a cleaner error message
		// to operators staring at config.
		err := b.CheckWritePermissions(ctx)
		require.ErrorIs(t, err, ErrMissingRequestingOrg)
	})
}

// TestBackend_ResourceNameUsesPerTenantPrefix verifies the bucket-layer
// isolation property: every object the backend reads or writes is
// addressed under the tenant's KeyPrefix. Two tenants pushing the same
// blob digest must produce distinct keys at the underlying bucket level.
//
// Uses stub Backend values directly because resourceName depends only
// on the creds field — no need to spin up SDK clients.
func TestBackend_ResourceNameUsesPerTenantPrefix(t *testing.T) {
	t.Parallel()

	bA := &Backend{creds: &Credentials{
		AccessPointARN: "arn:aws:s3:us-east-1:111:accesspoint/ap-a",
		KeyPrefix:      "org/A",
	}}
	bB := &Backend{creds: &Credentials{
		AccessPointARN: "arn:aws:s3:us-east-1:111:accesspoint/ap-b",
		KeyPrefix:      "org/B",
	}}

	digest := "deadbeef"
	keyA := bA.resourceName(digest)
	keyB := bB.resourceName(digest)
	assert.Equal(t, "org/A/sha256:deadbeef", keyA)
	assert.Equal(t, "org/B/sha256:deadbeef", keyB)
	assert.NotEqual(t, keyA, keyB, "same digest must produce distinct keys across tenants")
}

// TestSessionPolicy_ScopesToTenantPrefix locks down the session-policy
// generator: the IAM policy minted at AssumeRole time must reference
// both the AP ARN and the tenant key prefix, so a leaked token can't
// touch keys outside its tenant's namespace.
func TestSessionPolicy_ScopesToTenantPrefix(t *testing.T) {
	t.Parallel()

	policy := buildSessionPolicy("arn:aws:s3:us-east-1:111:accesspoint/ap-a", "org/A")

	assert.Contains(t, policy, `"arn:aws:s3:us-east-1:111:accesspoint/ap-a/object/org/A/*"`,
		"policy Resource must be the AP ARN + tenant prefix")
	assert.NotContains(t, policy, `"*"`,
		"session policy must not wildcard the Resource")
	assert.Contains(t, policy, `"s3:GetObject"`)
	assert.Contains(t, policy, `"s3:PutObject"`)
}

// TestRoleSessionName_DerivedFromOrg pins the session-name shape that
// the AP resource policy condition depends on. Changing the format here
// without updating the AP-side IaC will lock every tenant out.
func TestRoleSessionName_DerivedFromOrg(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "cas-abc-123", roleSessionName("abc-123"))
}

// --- helpers -----------------------------------------------------------

// newTestBackend constructs a fully wired *Backend that uses static dummy
// AWS credentials so LoadDefaultConfig doesn't reach out to IMDS/SSO. The
// resulting STS client would only be invoked if a test path slipped past
// the fail-closed guard — in which case the dummy creds would still
// trigger a fast, deterministic failure rather than a real AWS call.
func newTestBackend(t *testing.T) *Backend {
	t.Helper()
	return backendForCreds(t, &Credentials{
		AccessPointARN: "arn:aws:s3:us-east-1:123456789012:accesspoint/chainloop-org-abc",
		KeyPrefix:      "org/abc",
	})
}

func backendForCreds(t *testing.T, creds *Credentials) *Backend {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	// EC2_METADATA_SERVICE_ENDPOINT to a bogus host stops the SDK from
	// trying IMDS during config load when no static creds are picked
	// up — defensive in case the env-var pickup order changes.
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	b, err := NewBackend(context.Background(), &Config{
		BaseRoleARN:     "arn:aws:iam::123456789012:role/chainloop-cas-tenant",
		Region:          "us-east-1",
		SessionDuration: DefaultSessionDuration,
	}, creds)
	require.NoError(t, err)
	return b
}

// assertFailedClosed checks that an error originated from the
// missing-context guard, whether returned directly or wrapped by the
// AWS SDK credential-chain machinery.
func assertFailedClosed(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	require.Containsf(t, err.Error(), ErrMissingRequestingOrg.Error(),
		"expected fail-closed missing-org error, got %q", err)
}
