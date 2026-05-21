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

	"github.com/aws/aws-sdk-go-v2/aws"
	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	robotaccount "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackend_FailClosedWithoutRequestingOrg(t *testing.T) {
	b := newTestBackend(t)
	ctx := jwtmiddleware.NewContext(context.Background(), &robotaccount.Claims{StoredSecretID: "foo", BackendType: "BT", Role: robotaccount.Downloader})

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

// TestBackend_KeyDerivedFromRequestingOrg verifies the bucket-layer
// isolation property: every object the backend reads or writes is
// addressed under a prefix derived from the requesting org in ctx.
// One Backend invoked with two different ctx-orgs must produce distinct
// keys for the same digest, and an empty ctx must error out.
func TestBackend_KeyDerivedFromRequestingOrg(t *testing.T) {
	t.Parallel()

	b := &Backend{creds: &Credentials{
		AccessPointARN: "arn:aws:s3:us-east-1:111:accesspoint/ap-a",
	}}
	digest := "deadbeef"

	ctxA := jwtmiddleware.NewContext(context.Background(), &robotaccount.Claims{OrgID: "org-A", StoredSecretID: "foo", BackendType: "BT", Role: robotaccount.Downloader})
	keyA, err := b.keyFor(ctxA, digest)
	require.NoError(t, err)
	ctxB := jwtmiddleware.NewContext(context.Background(), &robotaccount.Claims{OrgID: "org-B", StoredSecretID: "foo", BackendType: "BT", Role: robotaccount.Downloader})
	keyB, err := b.keyFor(ctxB, digest)
	require.NoError(t, err)

	assert.Equal(t, "org-A/sha256:deadbeef", keyA)
	assert.Equal(t, "org-B/sha256:deadbeef", keyB)
	assert.NotEqual(t, keyA, keyB, "same digest must produce distinct keys across tenants")

	ctx := jwtmiddleware.NewContext(context.Background(), &robotaccount.Claims{StoredSecretID: "foo", BackendType: "BT", Role: robotaccount.Downloader})
	_, err = b.keyFor(ctx, digest)
	require.ErrorIs(t, err, ErrMissingRequestingOrg)
}

// TestSessionPolicy_ScopesToAccessPoint locks down the session-policy
// generator: the IAM policy minted at AssumeRole time must scope to the
// AP ARN (the tenant boundary lives in the AP resource policy, not
// here), and must not include S3 actions the backend never calls — that
// keeps the inline policy small so STS's packed-policy budget has
// headroom for tags inherited from the caller principal.
func TestSessionPolicy_ScopesToAccessPoint(t *testing.T) {
	t.Parallel()

	policy := buildSessionPolicy("arn:aws:s3:us-east-1:111:accesspoint/ap-a")

	assert.Contains(t, policy, `"arn:aws:s3:us-east-1:111:accesspoint/ap-a/object/*"`,
		"policy Resource must be the AP ARN scoped to /object/*")
	assert.NotContains(t, policy, `"*"`,
		"session policy must not wildcard the Resource")
	assert.Contains(t, policy, `"s3:GetObject"`)
	assert.Contains(t, policy, `"s3:PutObject"`)
	assert.NotContains(t, policy, `"s3:DeleteObject"`,
		"backend never deletes — re-adding s3:DeleteObject grows the packed-policy footprint without need")
	assert.NotContains(t, policy, `"s3:GetObjectAttributes"`,
		"backend never calls GetObjectAttributes — re-adding it grows the packed-policy footprint without need")
}

// TestRoleSessionName_DerivedFromOrg pins the session-name shape that
// the AP resource policy condition depends on. Changing the format here
// without updating the AP-side IaC will lock every tenant out.
func TestRoleSessionName_DerivedFromOrg(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "cas-abc-123", roleSessionName("abc-123"))
}

// TestSessionCredentialsProvider_DevModeShortCircuit verifies that the
// dev-mode bypass calls the ambient credentials provider instead of STS,
// and crucially that the missing-org fail-closed check still fires even
// in dev mode — so developers don't accidentally let an obvious bug
// through.
func TestSessionCredentialsProvider_DevModeShortCircuit(t *testing.T) {
	t.Parallel()

	ambient := &countingCredsProvider{
		creds: aws.Credentials{AccessKeyID: "AKDEV", SecretAccessKey: "secret", Source: "test"},
	}
	p := &sessionCredentialsProvider{
		ambientCreds:          ambient,
		useAmbientForRetrieve: true,
		creds: &Credentials{
			AccessPointARN: "arn:aws:s3:us-east-1:111:accesspoint/ap-a",
		},
		// stsClient deliberately nil; if dev mode short-circuits properly
		// it should never be touched. A non-nil pointer here would mask
		// regressions.
	}

	t.Run("returns ambient credentials when org is set", func(t *testing.T) {
		ctx := jwtmiddleware.NewContext(context.Background(), &robotaccount.Claims{OrgID: "org-A", StoredSecretID: "foo", BackendType: "BT", Role: robotaccount.Downloader})
		got, err := p.Retrieve(ctx)
		require.NoError(t, err)
		assert.Equal(t, "AKDEV", got.AccessKeyID)
		assert.Equal(t, 1, ambient.calls)
	})

	t.Run("still fails closed without requesting org", func(t *testing.T) {
		ambient.calls = 0
		_, err := p.Retrieve(jwtmiddleware.NewContext(context.Background(),
			&robotaccount.Claims{StoredSecretID: "foo", BackendType: "BT", Role: robotaccount.Downloader}))
		require.ErrorIs(t, err, ErrMissingRequestingOrg)
		assert.Equal(t, 0, ambient.calls, "ambient provider must not be hit when org is missing")
	})
}

// countingCredsProvider is the minimum aws.CredentialsProvider needed to
// observe whether the dev-mode short-circuit invoked it. Used in the
// dev-mode test above and nowhere else.
type countingCredsProvider struct {
	creds aws.Credentials
	calls int
}

func (c *countingCredsProvider) Retrieve(_ context.Context) (aws.Credentials, error) {
	c.calls++
	return c.creds, nil
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
		Region:         "us-east-1",
		BaseRoleARN:    "arn:aws:iam::123456789012:role/chainloop-cas-tenant",
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

	b, err := NewBackend(context.Background(), creds)
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
