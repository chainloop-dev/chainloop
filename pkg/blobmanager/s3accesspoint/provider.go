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

// Package s3accesspoint implements a CAS backend that targets a single AWS
// S3 Access Point per tenant. Multiple tenants share one physical bucket;
// per-tenant isolation is provided by:
//
//  1. The Access Point's resource policy, which gates who can address the AP
//     and may further restrict s3:prefix.
//  2. A per-request sts:AssumeRole that mints a scoped session whose
//     RoleSessionName is derived from the authenticated requesting org
//     (carried in the request context via WithRequestingOrg). The AP's
//     resource policy enforces a StringEquals on aws:userid so that a
//     session minted for org A cannot read or write to org B's AP — even if
//     org A's secret blob has been tampered with to point at org B's ARN.
//  3. A KeyPrefix that namespaces every object under <KeyPrefix>/sha256:<digest>
//     and is also referenced in the session policy's Resource field, so that
//     no tenant's request can address keys outside its own prefix.
//
// The session name MUST come from the request context, not from the secret
// blob: a secrets-store compromise alone must not let an attacker reroute
// uploads to another tenant's AP. See WithRequestingOrg.
package s3accesspoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
)

// ProviderID is the stable identifier used by the CASBackend table's enum
// and by every other place that needs to disambiguate this provider from
// the regular s3 one.
const ProviderID = "AWS-S3-ACCESS-POINT"

// DefaultSessionDuration is the STS token lifetime used when the deployment
// config doesn't specify one. STS allows up to 12h; 1h keeps blast radius
// of a leaked token small while still giving the credential cache useful
// reuse across consecutive uploads.
const DefaultSessionDuration = time.Hour

// Config carries the deployment-wide settings the provider needs to mint
// scoped per-tenant credentials. It does NOT contain AWS access keys — the
// pod's ambient IAM identity (IRSA / Pod Identity / instance profile /
// AWS_* env vars) is used to call sts:AssumeRole on BaseRoleARN.
type Config struct {
	// BaseRoleARN is the IAM role the controlplane / artifact-cas pod
	// assumes via STS at each upload/download. Its permission policy must
	// allow s3:{Get,Put,Delete,Head}Object against every access point in
	// the account; the per-call session policy narrows that down to one
	// AP + one prefix.
	BaseRoleARN string
	// Region is the default region for the underlying bucket and the
	// access points. Individual managed rows may override this via
	// Credentials.Region.
	Region string
	// SessionDuration is the STS token lifetime. Defaults to
	// DefaultSessionDuration when zero.
	SessionDuration time.Duration
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("s3accesspoint: nil config")
	}
	if c.BaseRoleARN == "" {
		return errors.New("s3accesspoint: base_role_arn is required")
	}
	if !strings.HasPrefix(c.BaseRoleARN, "arn:aws:iam::") {
		return fmt.Errorf("s3accesspoint: base_role_arn %q is not a valid IAM role ARN", c.BaseRoleARN)
	}
	if c.Region == "" {
		return errors.New("s3accesspoint: region is required")
	}
	return nil
}

// Credentials is the per-tenant blob stashed in the secrets manager under
// CASBackend.SecretName. Despite the name it carries no access keys — only
// tenant-identifying coordinates used to construct a scoped S3 client.
//
// The platform reconciler is responsible for writing this blob in lockstep
// with the AWS-side AP creation and policy.
type Credentials struct {
	// AccessPointARN, e.g.
	//   arn:aws:s3:us-east-1:123456789012:accesspoint/chainloop-org-<uuid>
	// The provider passes this string verbatim as the Bucket parameter on
	// every S3 SDK call.
	AccessPointARN string
	// Region overrides Config.Region for this tenant. Optional; useful if
	// the deployment grows multi-region without rolling a new config.
	Region string
	// KeyPrefix is the per-tenant key namespace inside the underlying
	// bucket, e.g. "org/<uuid>". The provider keys every object under
	// "<KeyPrefix>/sha256:<digest>" and the session policy's Resource is
	// scoped to "${apARN}/object/${KeyPrefix}/*". The AP's resource policy
	// is expected to enforce the same prefix via an s3:prefix condition.
	KeyPrefix string
}

func (c *Credentials) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: nil credentials", backend.ErrValidation)
	}
	if c.AccessPointARN == "" {
		return fmt.Errorf("%w: missing access_point_arn", backend.ErrValidation)
	}
	if !strings.HasPrefix(c.AccessPointARN, "arn:aws:s3:") || !strings.Contains(c.AccessPointARN, ":accesspoint/") {
		return fmt.Errorf("%w: access_point_arn %q is not an S3 access point ARN", backend.ErrValidation, c.AccessPointARN)
	}
	if c.KeyPrefix == "" {
		return fmt.Errorf("%w: missing key_prefix", backend.ErrValidation)
	}
	if strings.HasPrefix(c.KeyPrefix, "/") || strings.HasSuffix(c.KeyPrefix, "/") {
		return fmt.Errorf("%w: key_prefix %q must not start or end with '/'", backend.ErrValidation, c.KeyPrefix)
	}
	return nil
}

// BackendProvider implements backend.Provider for the access-point-backed
// managed CAS. Construction validates the deployment Config so a
// misconfigured controlplane fails at startup rather than at first upload.
type BackendProvider struct {
	cfg     *Config
	cReader credentials.Reader
}

var _ backend.Provider = (*BackendProvider)(nil)

// NewBackendProvider constructs the provider. It returns an error if cfg
// is missing required fields; callers (typically loader.LoadProviders) are
// expected to skip registration on error so on-prem deployments without
// managed CAS aren't affected.
func NewBackendProvider(cfg *Config, cReader credentials.Reader) (*BackendProvider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cReader == nil {
		return nil, errors.New("s3accesspoint: credentials reader is required")
	}
	// Normalize default session duration so downstream code can rely on a
	// non-zero value without re-checking everywhere.
	if cfg.SessionDuration == 0 {
		cfg.SessionDuration = DefaultSessionDuration
	}
	return &BackendProvider{cfg: cfg, cReader: cReader}, nil
}

func (p *BackendProvider) ID() string {
	return ProviderID
}

// FromCredentials reads the per-tenant Credentials blob from the secrets
// manager and constructs a *Backend bound to that tenant's AP.
//
// The returned UploaderDownloader is safe to reuse across requests; each
// request must enrich its context with WithRequestingOrg so the STS-minted
// session name matches the AP's resource-policy condition.
func (p *BackendProvider) FromCredentials(ctx context.Context, secretName string) (backend.UploaderDownloader, error) {
	creds := &Credentials{}
	if err := p.cReader.ReadCredentials(ctx, secretName, creds); err != nil {
		return nil, err
	}
	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials retrieved from storage: %w", err)
	}
	return NewBackend(ctx, p.cfg, creds)
}

// ValidateAndExtractCredentials decodes credsJSON into a Credentials struct
// and optionally cross-checks it against the location passed by the caller.
// This is invoked when a managed row is being created or revalidated; the
// returned value is what gets persisted in the secrets manager by upstream
// callers.
//
// Unlike the regular s3 provider, this does NOT exercise live S3
// permissions during validation: the credentials by themselves can't be
// tested without a request-context org UUID (see WithRequestingOrg), so a
// proper end-to-end check belongs in the upload path. PerformValidation in
// the controlplane still calls this method for managed rows; it will
// succeed as long as the blob is well-formed.
func (p *BackendProvider) ValidateAndExtractCredentials(location string, credsJSON []byte) (any, error) {
	var creds Credentials
	if err := json.Unmarshal(credsJSON, &creds); err != nil {
		return nil, fmt.Errorf("unmarshaling credentials: %w", err)
	}
	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}
	// If the caller supplied a location, it must agree with the blob.
	// This is a denormalization sanity check, not a security boundary —
	// the security boundary is the AP resource policy on the AWS side.
	if location != "" && location != creds.AccessPointARN {
		return nil, fmt.Errorf("%w: location %q does not match access_point_arn %q",
			backend.ErrValidation, location, creds.AccessPointARN)
	}
	return &creds, nil
}

// requestingOrgCtxKey is unexported so callers must go through
// WithRequestingOrg / requestingOrgFromContext; no risk of accidental
// collision with another package's keys.
type requestingOrgCtxKey struct{}

// WithRequestingOrg returns a derived context that carries the
// authenticated requesting organization's UUID. Every biz/service path
// that hands a managed-AP backend off to Upload/Download MUST enrich the
// ctx via this helper; without it the backend fails closed.
//
// The value MUST come from the request's authenticated identity (e.g.
// usercontext.CurrentOrg(ctx).ID), NOT from the resolved CASBackend or its
// secret blob. The whole secret-tampering defense depends on this being a
// source the attacker can't rewrite together with the secret store.
func WithRequestingOrg(ctx context.Context, orgUUID string) context.Context {
	return context.WithValue(ctx, requestingOrgCtxKey{}, orgUUID)
}

// requestingOrgFromContext extracts the requesting org UUID. Empty string
// means "no caller set the key" — the backend treats this as a hard error.
func requestingOrgFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestingOrgCtxKey{}).(string)
	return v
}
