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
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
)

const (
	annotationNameAuthor   = "author"
	annotationNameFilename = "filename"
)

// ErrMissingRequestingOrg is returned when a request reaches the backend
// without an org UUID in its context. The backend fails closed in this
// case rather than minting a session with a default/empty name that would
// be useless against an AP policy condition.
var ErrMissingRequestingOrg = errors.New("s3accesspoint: requesting org missing from context (call WithRequestingOrg before upload/download)")

// Backend is the per-tenant uploader/downloader. One *Backend instance is
// bound to one access point; the actual AWS credentials are minted
// per-request via STS using the org UUID found in the request context.
type Backend struct {
	cfg   *Config
	creds *Credentials

	// stsClient is built once at construction using the pod's ambient
	// IAM identity. The credential chain (IRSA → IMDS → env →
	// ~/.aws/credentials) picks up the identity automatically.
	stsClient *sts.Client

	// s3Client uses a custom CredentialsProvider that mints a scoped
	// session per request (cached in-process per requesting-org so back-
	// to-back uploads from the same org reuse the token). Bucket is
	// always the AP ARN; the SDK accepts an ARN there directly.
	s3Client *s3.Client
}

var _ backend.UploaderDownloader = (*Backend)(nil)

// NewBackend constructs a *Backend wired to an STS-backed credentials
// provider. ctx is used only for the initial AWS config load (DNS lookups,
// IMDS, IRSA token reads); it is not retained for later operations.
func NewBackend(ctx context.Context, cfg *Config, creds *Credentials) (*Backend, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := creds.Validate(); err != nil {
		return nil, err
	}

	region := cfg.Region
	if creds.Region != "" {
		region = creds.Region
	}

	// Load the pod's ambient AWS identity once. Subsequent SDK calls
	// reuse the resulting config; no per-request credential lookup
	// against the pod identity is necessary.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading aws config: %w", err)
	}

	stsClient := sts.NewFromConfig(awsCfg)

	// The per-request credential provider closes over cfg + creds so it
	// can build the session policy from the AP ARN and key prefix every
	// time AWS asks for fresh credentials. NewCredentialsCache handles
	// proactive refresh and concurrent-call deduplication.
	//
	// In dev mode we hand the provider the ambient credentials so it can
	// return them directly without calling STS. The provider still
	// enforces the requesting-org context discipline.
	credProvider := aws.NewCredentialsCache(&sessionCredentialsProvider{
		stsClient:             stsClient,
		ambientCreds:          awsCfg.Credentials,
		baseRoleARN:           cfg.BaseRoleARN,
		sessionDuration:       cfg.SessionDuration,
		useAmbientForRetrieve: cfg.DevModeUseAmbientCredentials,
		creds:                 creds,
	})

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.Credentials = credProvider
	})

	return &Backend{
		cfg:       cfg,
		creds:     creds,
		stsClient: stsClient,
		s3Client:  s3Client,
	}, nil
}

// keyFor builds the bucket-level S3 key for a digest. Every tenant's
// objects live under a prefix derived from the requesting org carried in
// ctx, so two tenants pushing the same digest don't collide at the bucket
// layer. The function fails closed when the org is missing — same
// invariant the credentials provider enforces, just surfaced earlier
// with a clearer error.
func (b *Backend) keyFor(ctx context.Context, digest string) (string, error) {
	orgUUID := backend.RequestingOrgFromContext(ctx)
	if orgUUID == "" {
		return "", ErrMissingRequestingOrg
	}
	return fmt.Sprintf("%s/sha256:%s", orgUUID, digest), nil
}

func (b *Backend) Exists(ctx context.Context, digest string) (bool, error) {
	_, err := b.Describe(ctx, digest)
	if err != nil && backend.IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

func (b *Backend) Upload(ctx context.Context, r io.Reader, resource *pb.CASResource) error {
	key, err := b.keyFor(ctx, resource.Digest)
	if err != nil {
		return err
	}
	uploader := manager.NewUploader(b.s3Client)
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.creds.AccessPointARN),
		Key:    aws.String(key),
		Body:   r,
		Metadata: map[string]string{
			annotationNameAuthor:   backend.AuthorAnnotation,
			annotationNameFilename: resource.FileName,
		},
	})
	if err != nil {
		return fmt.Errorf("uploading to access point: %w", err)
	}
	return nil
}

func (b *Backend) Describe(ctx context.Context, digest string) (*pb.CASResource, error) {
	key, err := b.keyFor(ctx, digest)
	if err != nil {
		return nil, err
	}
	resp, err := b.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       aws.String(b.creds.AccessPointARN),
		Key:          aws.String(key),
		ChecksumMode: s3types.ChecksumModeEnabled,
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			return nil, backend.NewErrNotFound("artifact")
		}
		return nil, fmt.Errorf("reading from access point: %w", err)
	}

	// Integrity check: when S3 returned a checksum, make sure the digest
	// the caller asked for matches the server's recorded value.
	if resp.ChecksumSHA256 != nil && *resp.ChecksumSHA256 != hexSha256ToBinaryB64(digest) {
		return nil, fmt.Errorf("failed to validate integrity of object, got=%s, want=%s",
			*resp.ChecksumSHA256, hexSha256ToBinaryB64(digest))
	}

	author, ok := resp.Metadata[annotationNameAuthor]
	if !ok || author != backend.AuthorAnnotation {
		return nil, errors.New("asset not uploaded by Chainloop")
	}
	fileName, ok := resp.Metadata[annotationNameFilename]
	if !ok {
		return nil, errors.New("couldn't find file metadata")
	}

	var size int64
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}
	return &pb.CASResource{FileName: fileName, Size: size, Digest: digest}, nil
}

func (b *Backend) Download(ctx context.Context, w io.Writer, digest string) error {
	exists, err := b.Exists(ctx, digest)
	if err != nil {
		return err
	} else if !exists {
		return backend.NewErrNotFound("artifact")
	}

	key, err := b.keyFor(ctx, digest)
	if err != nil {
		return err
	}
	downloader := manager.NewDownloader(b.s3Client, func(d *manager.Downloader) {
		// Force sequential downloads so the fakeWriterAt below can
		// safely ignore the offset argument.
		d.Concurrency = 1
	})
	_, err = downloader.Download(ctx, fakeWriterAt{w}, &s3.GetObjectInput{
		Bucket: aws.String(b.creds.AccessPointARN),
		Key:    aws.String(key),
	})
	return err
}

// CheckWritePermissions verifies that the calling org can actually mint a
// scoped session and put/get an object through its AP. Unlike the regular
// s3 backend's variant this MUST be invoked with a context carrying
// WithRequestingOrg; otherwise it fails closed.
func (b *Backend) CheckWritePermissions(ctx context.Context) error {
	orgUUID := backend.RequestingOrgFromContext(ctx)
	if orgUUID == "" {
		return ErrMissingRequestingOrg
	}
	const testObject = "healthcheck"
	key := fmt.Sprintf("%s/%s", orgUUID, testObject)

	if _, err := b.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Body:   strings.NewReader("healthcheckdata"),
		Bucket: aws.String(b.creds.AccessPointARN),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("writing healthcheck object: %w", err)
	}
	if _, err := b.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.creds.AccessPointARN),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("reading healthcheck object: %w", err)
	}
	return nil
}

// sessionCredentialsProvider implements aws.CredentialsProvider. Each
// Retrieve call extracts the requesting org from ctx, builds a session
// policy that scopes the resulting credentials to one AP + one key
// prefix, and calls sts:AssumeRole.
//
// The aws.NewCredentialsCache wrapper around this provider takes care of
// reusing the temporary credentials across consecutive calls until the
// expiration window approaches.
type sessionCredentialsProvider struct {
	stsClient       *sts.Client
	baseRoleARN     string
	sessionDuration time.Duration

	// ambientCreds is the SDK-default credentials provider captured from
	// awsCfg at construction time. Only consulted when
	// useAmbientForRetrieve is true (dev mode).
	ambientCreds aws.CredentialsProvider
	// useAmbientForRetrieve short-circuits Retrieve to return the pod's
	// ambient AWS credentials directly without calling sts:AssumeRole.
	// DEV ONLY — see Config.DevModeUseAmbientCredentials.
	useAmbientForRetrieve bool

	creds *Credentials
}

// Retrieve is called by the AWS SDK before every signed request. It must
// be cheap to call (the cache wrapper deduplicates concurrent misses and
// caches valid creds until ExpiresIn).
func (p *sessionCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	orgUUID := backend.RequestingOrgFromContext(ctx)
	if orgUUID == "" {
		return aws.Credentials{}, ErrMissingRequestingOrg
	}

	// Dev mode: skip the per-request AssumeRole entirely and use the
	// SDK's default credential chain directly. We still required the
	// org-from-ctx check above so callers that forget WithRequestingOrg
	// fail the same way they would in production.
	if p.useAmbientForRetrieve {
		if p.ambientCreds == nil {
			return aws.Credentials{}, errors.New("s3accesspoint: dev mode requested but no ambient credentials available")
		}
		return p.ambientCreds.Retrieve(ctx)
	}

	// Session policy intersects with the base role's permissions; even
	// if the role grants accesspoint/*, this session can only touch the
	// caller's AP and prefix. The prefix is the requesting-org UUID
	// straight from ctx — same source as the session name — so a
	// tampered AccessPointARN in the secret blob can't widen the prefix
	// scope to escape into another tenant's namespace.
	sessionPolicy := buildSessionPolicy(p.creds.AccessPointARN, orgUUID)

	durSecs := int32(p.sessionDuration / time.Second)
	if durSecs <= 0 {
		durSecs = int32(DefaultSessionDuration / time.Second)
	}

	out, err := p.stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         aws.String(p.baseRoleARN),
		RoleSessionName: aws.String(roleSessionName(orgUUID)),
		Policy:          aws.String(sessionPolicy),
		DurationSeconds: aws.Int32(durSecs),
	})
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("sts:AssumeRole for org %s: %w", orgUUID, err)
	}
	if out.Credentials == nil {
		return aws.Credentials{}, errors.New("sts:AssumeRole returned no credentials")
	}

	return aws.Credentials{
		AccessKeyID:     aws.ToString(out.Credentials.AccessKeyId),
		SecretAccessKey: aws.ToString(out.Credentials.SecretAccessKey),
		SessionToken:    aws.ToString(out.Credentials.SessionToken),
		Source:          "s3accesspoint",
		CanExpire:       true,
		Expires:         aws.ToTime(out.Credentials.Expiration),
	}, nil
}

// roleSessionName binds the AssumeRole session to the requesting org.
// AWS limits session names to 64 chars and a restricted character set; a
// "cas-<orgUUID>" string is well within that.
func roleSessionName(orgUUID string) string {
	return "cas-" + orgUUID
}

// buildSessionPolicy returns an IAM policy document that allows only the
// operations the backend actually performs, and only against this
// tenant's AP + key prefix. The Resource ARNs use the AP form
// "${apARN}/object/${keyPrefix}/*".
func buildSessionPolicy(apARN, keyPrefix string) string {
	// Minimal, hand-written JSON — keeping it small reduces request
	// payload (STS limits session policies to 2048 chars by default).
	return fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:PutObject","s3:DeleteObject","s3:GetObjectAttributes"],"Resource":"%s/object/%s/*"}]}`,
		apARN, keyPrefix)
}

// hexSha256ToBinaryB64 decodes the hex sha and re-encodes as base64. S3
// returns the recorded checksum in base64 form; comparing it to a hex
// digest needs this conversion.
func hexSha256ToBinaryB64(hexString string) string {
	decoded, err := hex.DecodeString(hexString)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(decoded)
}

// fakeWriterAt wraps an io.Writer so the SDK's WriterAt-shaped
// downloader can be driven by a regular writer. Safe only when
// concurrency is forced to 1.
type fakeWriterAt struct {
	w io.Writer
}

func (fw fakeWriterAt) WriteAt(p []byte, _ int64) (int, error) {
	return fw.w.Write(p)
}
