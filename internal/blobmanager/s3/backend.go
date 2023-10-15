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
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
)

type Backend struct {
	client *s3.S3
	bucket string
}

var _ backend.UploaderDownloader = (*Backend)(nil)

func NewBackend(creds *Credentials) (*Backend, error) {
	c := credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, "")
	session, err := session.NewSession(&aws.Config{Credentials: c, Region: aws.String(creds.Region)})
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
	return false, errors.New("not implemented")
}

func (b *Backend) Upload(ctx context.Context, r io.Reader, resource *pb.CASResource) error {
	return errors.New("not implemented")
}

func (b *Backend) Describe(ctx context.Context, digest string) (*pb.CASResource, error) {
	return nil, errors.New("not implemented")
}

func (b *Backend) Download(ctx context.Context, w io.Writer, digest string) error {
	return errors.New("not implemented")
}

// CheckWritePermissions performs an actual write to the repository to check that the credentials
func (b *Backend) CheckWritePermissions(ctx context.Context) error {
	testObject := "healthcheck"
	testObjectData := "healthcheckdata"

	// We use the sha256 hash of the test object integrity during uploads and downloads
	hash := sha256.New()
	if _, err := io.WriteString(hash, testObjectData); err != nil {
		return fmt.Errorf("failed to write to hash: %w", err)
	}

	hashBytes := hash.Sum(nil)
	hashString := base64.StdEncoding.EncodeToString(hashBytes)
	fmt.Println(hashString)

	input := &s3.PutObjectInput{
		Body:           aws.ReadSeekCloser(strings.NewReader(testObjectData)),
		Bucket:         aws.String(b.bucket),
		Key:            aws.String(testObject),
		ChecksumSHA256: aws.String(hashString),
	}

	// Write to the bucket
	if _, err := b.client.PutObjectWithContext(ctx, input); err != nil {
		return fmt.Errorf("failed to write to bucket: %w", err)
	}

	// and read the object back + validate integrity
	resp, err := b.client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket:       aws.String(b.bucket),
		Key:          aws.String(testObject),
		ChecksumMode: aws.String("ENABLED"),
	})

	if err != nil {
		return fmt.Errorf("failed to read from bucket: %w", err)
	}

	if resp.ChecksumSHA256 != nil && *resp.ChecksumSHA256 != hashString {
		return fmt.Errorf("failed to validate integrity of object")
	}

	return nil
}
