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
//     RoleSessionName is derived from the authenticated requesting org. The AP's
//     resource policy enforces a StringEquals on aws:userid so that a
//     session minted for org A cannot read or write to org B's AP — even if
//     org A's secret blob has been tampered with to point at org B's ARN.
//  3. A per-tenant key prefix derived from the requesting org UUID: every
//     object is keyed as <orgUUID>/sha256:<digest> and the AssumeRole
//     session policy's Resource is scoped to ${apARN}/object/<orgUUID>/*.
//     The prefix shares its source of truth with the session name, so a
//     tampered secret cannot reroute a tenant's writes into a different
//     namespace.
//
// The session name MUST come from the request context, not from the secret
// blob: a secrets-store compromise alone must not let an attacker reroute
// uploads to another tenant's AP.
package s3accesspoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
)

// ProviderID is the stable identifier used by the CASBackend table's enum
// and by every other place that needs to disambiguate this provider from
// the regular s3 one.
const ProviderID = "AWS-S3-ACCESS-POINT"

// SessionDuration is the STS token lifetime. STS allows up to 12h; 1h keeps
// blast radius of a leaked token small while still giving the credential
// cache useful reuse across consecutive uploads.
const SessionDuration = time.Hour

// DevModeEnvVar when set to a truthy value, short-circuits sts:AssumeRole
// and routes S3 calls through whatever ambient AWS identity the SDK's
// default credential chain produced (env vars, ~/.aws/credentials, instance
// profile, IRSA, …). The fail-closed check on a missing requesting-org
// context is still enforced.
//
// DEV ONLY. This bypasses the per-tenant isolation guarantees that the
// AssumeRole + session-policy + AP-policy chain provides; objects
// addressed via this backend are limited only by whatever the developer's
// IAM identity allows. NEVER set this in a multi-tenant deployment.
const DevModeEnvVar = "CHAINLOOP_S3_ACCESS_POINT_DEV_MODE"

// devModeEnabled reads DevModeEnvVar and returns true for the usual truthy
// spellings. Kept as a package-level function so tests can swap the env
// var with t.Setenv.
func devModeEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(DevModeEnvVar)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Credentials is the per-tenant blob stashed in the secrets manager under
// CASBackend.SecretName. Despite the name it carries no access keys — only
// tenant-identifying coordinates used to construct a scoped S3 client.
//
// The per-tenant key prefix is intentionally NOT a field here: it's
// derived at request time from the authenticated requesting org carried
// in ctx via org claim. Both the bucket-layer key namespace and
// the AssumeRole session-name binding therefore come from the same
// untamperable source, so a secrets-store compromise that rewrites this
// blob still can't reroute a tenant's writes into another tenant's
// namespace.
type Credentials struct {
	// AccessPointARN, e.g.
	//   arn:aws:s3:us-east-1:123456789012:accesspoint/chainloop-org-<uuid>
	// The provider passes this string verbatim as the Bucket parameter on
	// every S3 SDK call.
	AccessPointARN string
	// Region the AP lives in.
	Region string
	// BaseRoleARN is the IAM role assumed via STS to mint per-request,
	// per-tenant scoped credentials. Stored per-tenant (not per-deployment)
	// so a single chainloop install can serve tenants across multiple AWS
	// accounts without a config change. Required unless DevModeEnvVar is
	// set on the running binary.
	BaseRoleARN string
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
	if c.Region == "" {
		return fmt.Errorf("%w: missing region", backend.ErrValidation)
	}
	if !devModeEnabled() {
		if c.BaseRoleARN == "" {
			return fmt.Errorf("%w: missing base_role_arn", backend.ErrValidation)
		}
		if !strings.HasPrefix(c.BaseRoleARN, "arn:aws:iam::") {
			return fmt.Errorf("%w: base_role_arn %q is not a valid IAM role ARN", backend.ErrValidation, c.BaseRoleARN)
		}
	}
	return nil
}

// BackendProvider implements backend.Provider for the access-point-backed
// managed CAS. Construction takes only the credentials reader; everything
// the provider needs at request time lives in the per-tenant secret blob.
type BackendProvider struct {
	cReader credentials.Reader
}

var _ backend.Provider = (*BackendProvider)(nil)

// NewBackendProvider constructs the provider. A nil credentials reader is
// a programmer error and surfaces as a startup failure.
func NewBackendProvider(cReader credentials.Reader) (*BackendProvider, error) {
	if cReader == nil {
		return nil, errors.New("s3accesspoint: credentials reader is required")
	}
	if devModeEnabled() {
		log.Printf("WARNING: s3accesspoint provider running with %s=true; sts:AssumeRole is bypassed and per-tenant isolation is NOT enforced — DEV USE ONLY", DevModeEnvVar)
	}
	return &BackendProvider{cReader: cReader}, nil
}

func (p *BackendProvider) ID() string {
	return ProviderID
}

// FromCredentials reads the per-tenant Credentials blob from the secrets
// manager and constructs a *Backend bound to that tenant's AP.
//
// The returned UploaderDownloader is safe to reuse across requests; each
// request must enrich its context with org claim so the STS-minted
// session name matches the AP's resource-policy condition.
func (p *BackendProvider) FromCredentials(ctx context.Context, secretName string) (backend.UploaderDownloader, error) {
	creds := &Credentials{}
	if err := p.cReader.ReadCredentials(ctx, secretName, creds); err != nil {
		return nil, err
	}
	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials retrieved from storage: %w", err)
	}
	return NewBackend(ctx, creds)
}

// ValidateAndExtractCredentials decodes credsJSON into a Credentials struct
// and optionally cross-checks it against the location passed by the caller.
// This is invoked when a managed row is being created or revalidated; the
// returned value is what gets persisted in the secrets manager by upstream
// callers.
//
// Unlike the regular s3 provider, this does NOT exercise live S3
// permissions during validation: the credentials by themselves can't be
// tested without a request-context org UUID, so a
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
	if location != "" && location != creds.AccessPointARN {
		return nil, fmt.Errorf("%w: location %q does not match access_point_arn %q",
			backend.ErrValidation, location, creds.AccessPointARN)
	}
	return &creds, nil
}
