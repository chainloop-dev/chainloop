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
	"fmt"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/credentials/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		name    string
		creds   *Credentials
		wantErr bool
	}{
		{
			name: "valid credentials",
			creds: &Credentials{
				AccessKeyID:     "test",
				SecretAccessKey: "test",
				Region:          "test",
				Location:        "test",
			},
		},
		{
			name: "valid credentials with deprecated bucket name",
			creds: &Credentials{
				AccessKeyID:     "test",
				SecretAccessKey: "test",
				Region:          "test",
				BucketName:      "test",
			},
		},
		{
			name: "missing access key id",
			creds: &Credentials{
				SecretAccessKey: "test",
				Region:          "test",
				BucketName:      "test",
			},
			wantErr: true,
		},
		{
			name: "missing secret access key",
			creds: &Credentials{
				AccessKeyID: "test",
				Region:      "test",
				BucketName:  "test",
			},
			wantErr: true,
		},
		{
			name: "missing bucket name",
			creds: &Credentials{
				AccessKeyID:     "test",
				SecretAccessKey: "test",
				Region:          "test",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.creds.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFromCredentials(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)
	r := mocks.NewReader(t)
	const bucket, keyID, keySecret, region = "my-bucket", "key-id", "key-secret", "region-1"

	t.Run("with deprecated bucketName", func(_ *testing.T) {
		r.On("ReadCredentials", ctx, "secretName", mock.AnythingOfType("*s3.Credentials")).Return(nil).Run(
			func(args mock.Arguments) {
				credentials := args.Get(2).(*Credentials)
				credentials.BucketName = bucket
				credentials.Region = region
				credentials.SecretAccessKey = keySecret
				credentials.AccessKeyID = keyID
			})

		_, err := NewBackendProvider(r).FromCredentials(ctx, "secretName")
		assert.NoError(err)
	})

	t.Run("with location", func(_ *testing.T) {
		r.On("ReadCredentials", ctx, "secretName", mock.AnythingOfType("*s3.Credentials")).Return(nil).Run(
			func(args mock.Arguments) {
				credentials := args.Get(2).(*Credentials)
				credentials.Location = fmt.Sprintf("https://123.r2.cloudflarestorage.com/%s", bucket)
				credentials.Region = region
				credentials.SecretAccessKey = keySecret
				credentials.AccessKeyID = keyID
			})

		_, err := NewBackendProvider(r).FromCredentials(ctx, "secretName")
		assert.NoError(err)
	})
}

func TestExtractCreds(t *testing.T) {
	tetCases := []struct {
		name      string
		location  string
		credsJSON []byte
		wantErr   bool
	}{
		{
			name:     "valid credentials",
			location: "mybucket",
			credsJSON: []byte(`{
				"AccessKeyID": "keyID",
				"SecretAccessKey": "keySecret",
				"Region": "region-1"
			}`),
		},
		{
			name:     "invalid location, missing bucket",
			location: "",
			wantErr:  true,
			credsJSON: []byte(`{
				"AccessKeyID": "test",
				"SecretAccessKey": "keySecret",
				"Region": "test"
			}`),
		},
		{
			name:     "invalid credentials, missing secret",
			location: "account/container",
			credsJSON: []byte(`{
				"AccessKeyID": "test",
				"Region": "region-1"
			}`),
			wantErr: true,
		},
	}

	for _, tc := range tetCases {
		t.Run(tc.name, func(t *testing.T) {
			creds, err := extractCreds(tc.location, tc.credsJSON)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, &Credentials{
					Region:          "region-1",
					SecretAccessKey: "keySecret",
					AccessKeyID:     "keyID",
					Location:        tc.location,
				}, creds)
			}
		})
	}
}

func TestProviderID(t *testing.T) {
	assert.Equal(t, "AWS-S3", NewBackendProvider(nil).ID())
}
