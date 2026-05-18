//
// Copyright 2023 The Chainloop Authors.
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

package backend

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

const (
	AuthorAnnotation = "chainloop.dev"
	// Default prefix for the blobmanager
	DefaultPrefix = "chainloop"
)

type Uploader interface {
	Upload(ctx context.Context, r io.Reader, resource *v1.CASResource) error
	Exists(ctx context.Context, digest string) (bool, error)
	CheckWritePermissions(ctx context.Context) error
}

type UploaderDownloader interface {
	Uploader
	Downloader
	Describer
}

type Describer interface {
	Describe(ctx context.Context, digest string) (*v1.CASResource, error)
}

type Downloader interface {
	Download(ctx context.Context, w io.Writer, digest string) error
}

var ErrValidation = errors.New("credentials validation error")

// Provider is an interface that allows to create a backend from a secret
type Provider interface {
	// Provider identifier
	ID() string
	// retrieve a downloader/uploader from a secret
	FromCredentials(ctx context.Context, secretName string) (UploaderDownloader, error)
	// validate and extract credentials from raw json
	ValidateAndExtractCredentials(location string, credsJSON []byte) (any, error)
}

type Providers map[string]Provider

// Detect the media type based on the provided content
func DetectedMediaType(b []byte) types.MediaType {
	return types.MediaType(strings.Split(http.DetectContentType(b), ";")[0])
}

// requestingOrgCtxKey is unexported so callers must go through
// WithRequestingOrg / RequestingOrgFromContext; no risk of accidental
// collision with another package's keys.
type requestingOrgCtxKey struct{}

// WithRequestingOrg returns a derived context that carries the
// authenticated requesting organization's UUID. Managed backends
// (currently AWS-S3-ACCESS-POINT) consume this value to scope per-
// tenant STS sessions; non-managed backends ignore it.
//
// The value MUST come from the verified caller identity (e.g. a CAS
// JWT claim), NOT from a resolved CASBackend row or its secret blob.
// The whole secret-tampering defense for managed CAS depends on this
// being a source the attacker can't rewrite together with the secret
// store.
//
// Callers typically set this once at the auth boundary (the CAS
// server's JWT middleware) and let the value flow through ctx into
// the backend's request handlers.
func WithRequestingOrg(ctx context.Context, orgUUID string) context.Context {
	return context.WithValue(ctx, requestingOrgCtxKey{}, orgUUID)
}

// RequestingOrgFromContext extracts the requesting org UUID previously
// stamped by WithRequestingOrg. Empty string means "no caller set the
// key" — backends that need a tenant identifier (e.g. managed CAS)
// should treat that as a fail-closed condition.
func RequestingOrgFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestingOrgCtxKey{}).(string)
	return v
}
