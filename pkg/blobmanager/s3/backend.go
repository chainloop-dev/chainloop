//
// Copyright 2024 The Chainloop Authors.
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

package s3

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
)

const (
	annotationNameAuthor   = "Author"
	annotationNameFilename = "Filename"
)

type Backend struct {
	client         *s3.S3
	bucket         string
	customEndpoint string
}

var _ backend.UploaderDownloader = (*Backend)(nil)

const defaultRegion = "us-east-1"

func NewBackend(creds *Credentials) (*Backend, error) {
	if creds == nil {
		return nil, errors.New("credentials cannot be nil")
	}

	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	// Set a default region if not provided
	var region = defaultRegion
	if creds.Region != "" {
		region = creds.Region
	}

	c := credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, "")
	// Configure AWS session
	cfg := &aws.Config{Credentials: c, Region: aws.String(region)}

	// Bucket might contain the not only the bucket name but also the endpoint
	endpoint, bucket, err := extractLocationAndBucket(creds)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bucket name: %w", err)
	}

	// we have a custom endpoint
	// in some cases the server-side checksum verification is not supported like in the case of cloudflare r2
	if endpoint != "" {
		cfg.Endpoint = aws.String(endpoint)
		cfg.S3ForcePathStyle = aws.Bool(true)
	}

	session, err := session.NewSession(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &Backend{
		client:         s3.New(session),
		bucket:         bucket,
		customEndpoint: endpoint,
	}, nil
}

// For now we are aware that the checksum verification is not supported by cloudflare r2
// https://developers.cloudflare.com/r2/api/s3/api/
func (b *Backend) checksumVerificationEnabled() bool {
	var enabled = true
	if b.customEndpoint != "" && strings.Contains(b.customEndpoint, "r2.cloudflarestorage.com") {
		enabled = false
	}

	return enabled
}

// Extract the custom endpoint and the bucket name from the location string
// The location string can be either a bucket name or a URL
// i.e bucket-name or https://custom-domain/bucket-name
func extractLocationAndBucket(creds *Credentials) (string, string, error) {
	// Older versions of the credentials didn't have the location field
	// and just the bucket name was stored in the bucket name field
	if creds.BucketName != "" {
		return "", creds.BucketName, nil
	}

	// Newer versions of the credentials have the location field which can contain the endpoint
	// so we override the bucket and set the endpoint if needed
	parsedLocation, err := url.Parse(creds.Location)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse location: %w", err)
	}

	host := parsedLocation.Host
	// It's a bucket name
	if host == "" {
		return "", creds.Location, nil
	}

	endpoint := fmt.Sprintf("%s://%s", parsedLocation.Scheme, host)
	// It's a URL, extract bucket name from the path
	if pathSegments := strings.Split(parsedLocation.Path, "/"); len(pathSegments) > 1 {
		return endpoint, pathSegments[1], nil
	}

	return "", "", fmt.Errorf("the location doesn't contain a bucket name")
}

// Exists check that the artifact is already present in the repository
func (b *Backend) Exists(ctx context.Context, digest string) (bool, error) {
	_, err := b.Describe(ctx, digest)
	if err != nil && backend.IsNotFound(err) {
		return false, nil
	}

	return err == nil, err
}

func (b *Backend) Upload(ctx context.Context, r io.Reader, resource *pb.CASResource) error {
	uploader := s3manager.NewUploaderWithClient(b.client)
	input := &s3manager.UploadInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(resourceName(resource.Digest)),
		Body:   r,
		Metadata: map[string]*string{
			annotationNameAuthor:   aws.String(backend.AuthorAnnotation),
			annotationNameFilename: aws.String(resource.FileName),
		},
	}

	if b.checksumVerificationEnabled() {
		// Check that the object is uploaded correctly
		input.ChecksumSHA256 = aws.String(hexSha256ToBinaryB64(resource.Digest))
	}

	if _, err := uploader.UploadWithContext(ctx, input); err != nil {
		return fmt.Errorf("failed to upload to bucket: %w", err)
	}

	return nil
}

func (b *Backend) Describe(ctx context.Context, digest string) (*pb.CASResource, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(resourceName(digest)),
	}

	if b.checksumVerificationEnabled() {
		// Enable checksum verification
		input.ChecksumMode = aws.String("ENABLED")
	}

	// and read the object back
	resp, err := b.client.HeadObjectWithContext(ctx, input)

	// check error is aws error
	var awsErr awserr.Error
	if err != nil {
		if errors.As(err, &awsErr) && awsErr.Code() == "NotFound" {
			return nil, backend.NewErrNotFound("artifact")
		}

		return nil, fmt.Errorf("failed to read from bucket: %w", err)
	}

	// Check integrity of the remote object
	if resp.ChecksumSHA256 != nil && *resp.ChecksumSHA256 != hexSha256ToBinaryB64(digest) {
		return nil, fmt.Errorf("failed to validate integrity of object, got=%s, want=%s", *resp.ChecksumSHA256, hexSha256ToBinaryB64(digest))
	}

	// Check asset author is Chainloop that way we can ignore files uploaded by other tools
	// note: this is not a security mechanism, an additional check will be put in place for tamper check
	author, ok := resp.Metadata[annotationNameAuthor]
	if !ok || *author != backend.AuthorAnnotation {
		return nil, errors.New("asset not uploaded by Chainloop")
	}

	fileName, ok := resp.Metadata[annotationNameFilename]
	if !ok {
		return nil, fmt.Errorf("couldn't find file metadata")
	}

	return &pb.CASResource{
		FileName: *fileName,
		Size:     *resp.ContentLength,
		Digest:   digest,
	}, nil
}

func (b *Backend) Download(ctx context.Context, w io.Writer, digest string) error {
	exists, err := b.Exists(ctx, digest)
	if err != nil {
		return err
	} else if !exists {
		return backend.NewErrNotFound("artifact")
	}

	downloader := s3manager.NewDownloaderWithClient(b.client)
	// force sequential downloads so we can wrap the writer and ignore the offset
	// Important! Do not change this value, otherwise the fakeWriterAt will not work
	downloader.Concurrency = 1
	output := fakeWriterAt{w}

	_, err = downloader.DownloadWithContext(ctx, output, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(resourceName(digest)),
	})

	return err
}

// CheckWritePermissions performs an actual write to the repository to check that the credentials
func (b *Backend) CheckWritePermissions(ctx context.Context) error {
	testObject := "healthcheck"

	input := &s3.PutObjectInput{
		Body:   aws.ReadSeekCloser(strings.NewReader("healthcheckdata")),
		Bucket: aws.String(b.bucket),
		Key:    aws.String(testObject),
	}

	// Write to the bucket
	if _, err := b.client.PutObjectWithContext(ctx, input); err != nil {
		return fmt.Errorf("failed to write to bucket: %w", err)
	}

	// and read the object back
	_, err := b.client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(testObject),
	})

	if err != nil {
		return fmt.Errorf("failed to read from bucket: %w", err)
	}

	return nil
}

// decode the hex string into a []byte slice.
// base64 encode the result
func hexSha256ToBinaryB64(hexString string) string {
	// Decode the hex string into a []byte slice.
	decoded, err := hex.DecodeString(hexString)
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(decoded)
}

func resourceName(digest string) string {
	return fmt.Sprintf("sha256:%s", digest)
}

// fakeWriterAt is a wrapper around io.Writer that ignores the offset
// we have this wrapper as a compatibility bridge between the backend.Downloader and io.WriterAt
// This is ok since we force sequential downloads with concurrency=1
type fakeWriterAt struct {
	w io.Writer
}

func (fw fakeWriterAt) WriteAt(p []byte, _ int64) (n int, err error) {
	// ignore 'offset' because we forced sequential downloads
	return fw.w.Write(p)
}
