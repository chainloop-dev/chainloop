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

package s3

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
)

const (
	annotationNameAuthor   = "Author"
	annotationNameFilename = "Filename"
)

type Backend struct {
	client *s3.S3
	bucket string
}

var _ backend.UploaderDownloader = (*Backend)(nil)

type ConnOpt func(*aws.Config)

// Optional endpoint configuration
func WithEndpoint(endpoint string) ConnOpt {
	return func(cfg *aws.Config) {
		cfg.Endpoint = aws.String(endpoint)
	}
}

func WithForcedS3PathStyle(force bool) ConnOpt {
	return func(cfg *aws.Config) {
		cfg.S3ForcePathStyle = aws.Bool(force)
	}
}

func NewBackend(creds *Credentials, connOpts ...ConnOpt) (*Backend, error) {
	if creds == nil {
		return nil, errors.New("credentials cannot be nil")
	}

	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	c := credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, "")
	// Configure AWS session
	cfg := &aws.Config{Credentials: c, Region: aws.String(creds.Region)}
	for _, opt := range connOpts {
		opt(cfg)
	}

	session, err := session.NewSession(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &Backend{
		client: s3.New(session),
		bucket: creds.BucketName,
	}, nil
}

// Exists check that the artifact is already present in the repository
func (b *Backend) Exists(ctx context.Context, digest string) (bool, error) {
	_, err := b.Describe(ctx, digest)
	notFoundErr := &backend.ErrNotFound{}
	if err != nil && errors.As(err, &notFoundErr) {
		return false, nil
	}

	return err == nil, err
}

func (b *Backend) Upload(ctx context.Context, r io.Reader, resource *pb.CASResource) error {
	uploader := s3manager.NewUploaderWithClient(b.client)

	_, err := uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(resourceName(resource.Digest)),
		Body:   r,
		// Check that the object is uploaded correctly
		ChecksumSHA256: aws.String(hexSha256ToBinaryB64(resource.Digest)),
		Metadata: map[string]*string{
			annotationNameAuthor:   aws.String(backend.AuthorAnnotation),
			annotationNameFilename: aws.String(resource.FileName),
		},
	})

	return err
}

func (b *Backend) Describe(ctx context.Context, digest string) (*pb.CASResource, error) {
	// and read the object back + validate integrity
	resp, err := b.client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket:       aws.String(b.bucket),
		Key:          aws.String(resourceName(digest)),
		ChecksumMode: aws.String("ENABLED"),
	})

	// check error is aws error
	var awsErr awserr.Error
	if err != nil {
		if errors.As(err, &awsErr) && awsErr.Code() == "NotFound" {
			return nil, &backend.ErrNotFound{}
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
