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

package azureblob

import (
	"context"
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
				StorageAccountName: "test",
				Container:          "test",
				TenantID:           "test",
				ClientID:           "test",
				ClientSecret:       "test",
			},
		},
		{
			name: "missing storage account",
			creds: &Credentials{
				StorageAccountName: "",
				Container:          "test",
				TenantID:           "test",
				ClientID:           "test",
				ClientSecret:       "test",
			},
			wantErr: true,
		},
		{
			name: "missing container",
			creds: &Credentials{
				StorageAccountName: "test",
				Container:          "",
				TenantID:           "test",
				ClientID:           "test",
				ClientSecret:       "test",
			},
			wantErr: true,
		},
		{
			name: "missing tenant id",
			creds: &Credentials{
				StorageAccountName: "test",
				Container:          "test",
				TenantID:           "",
				ClientID:           "test",
				ClientSecret:       "test",
			},
			wantErr: true,
		},
		{
			name: "missing client id",
			creds: &Credentials{
				StorageAccountName: "test",
				Container:          "test",
				TenantID:           "test",
				ClientID:           "",
				ClientSecret:       "test",
			},
			wantErr: true,
		},
		{
			name: "missing client secret",
			creds: &Credentials{
				StorageAccountName: "test",
				Container:          "test",
				TenantID:           "test",
				ClientID:           "test",
				ClientSecret:       "",
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
	const storageAccount, container, tenant, clientID, clientSecret = "storage", "container", "container", "clientID", "clientSecret"

	r.On("ReadCredentials", ctx, "secretName", mock.AnythingOfType("*azureblob.Credentials")).Return(nil).Run(
		func(args mock.Arguments) {
			credentials := args.Get(2).(*Credentials)
			credentials.StorageAccountName = storageAccount
			credentials.Container = container
			credentials.TenantID = tenant
			credentials.ClientID = clientID
			credentials.ClientSecret = clientSecret
		})

	_, err := NewBackendProvider(r).FromCredentials(ctx, "secretName")
	assert.NoError(err)
}

func TestExtractCreds(t *testing.T) {
	testCases := []struct {
		name      string
		location  string
		credsJSON []byte
		wantCreds *Credentials
		wantErr   bool
	}{
		{
			name:     "valid credentials without endpoint",
			location: "account/container",
			credsJSON: []byte(`{
				"storageAccountName": "test",
				"container": "test",
				"tenantID": "test",
				"clientID": "test",
				"clientSecret": "test"
			}`),
			wantCreds: &Credentials{
				StorageAccountName: "account",
				Container:          "container",
				TenantID:           "test",
				ClientID:           "test",
				ClientSecret:       "test",
				Endpoint:           "",
			},
		},
		{
			name:     "valid credentials with custom endpoint",
			location: "blob.core.usgovcloudapi.net/account/container",
			credsJSON: []byte(`{
				"storageAccountName": "test",
				"container": "test",
				"tenantID": "test",
				"clientID": "test",
				"clientSecret": "test"
			}`),
			wantCreds: &Credentials{
				StorageAccountName: "account",
				Container:          "container",
				TenantID:           "test",
				ClientID:           "test",
				ClientSecret:       "test",
				Endpoint:           "blob.core.usgovcloudapi.net",
			},
		},
		{
			name:     "invalid location, missing container",
			location: "account",
			wantErr:  true,
			credsJSON: []byte(`{
				"storageAccountName": "test",
				"container": "test",
				"tenantID": "test",
				"clientID": "test",
				"clientSecret": ""
			}`),
		},
		{
			name:     "invalid credentials, missing secret",
			location: "account/container",
			credsJSON: []byte(`{
				"storageAccountName": "test",
				"container": "test",
				"tenantID": "test",
				"clientID": "test",
				"clientSecret": ""
			}`),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			creds, err := extractCreds(tc.location, tc.credsJSON)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantCreds, creds)
			}
		})
	}
}

func TestExtractLocationAndContainer(t *testing.T) {
	testCases := []struct {
		name          string
		location      string
		wantEndpoint  string
		wantAccount   string
		wantContainer string
		wantErr       bool
	}{
		{
			name:          "simple location without endpoint",
			location:      "myaccount/mycontainer",
			wantEndpoint:  "",
			wantAccount:   "myaccount",
			wantContainer: "mycontainer",
		},
		{
			name:          "Azure Government Cloud endpoint",
			location:      "blob.core.usgovcloudapi.net/myaccount/mycontainer",
			wantEndpoint:  "blob.core.usgovcloudapi.net",
			wantAccount:   "myaccount",
			wantContainer: "mycontainer",
		},
		{
			name:          "Azure Stack Hub endpoint",
			location:      "blob.local.azurestack.external/myaccount/mycontainer",
			wantEndpoint:  "blob.local.azurestack.external",
			wantAccount:   "myaccount",
			wantContainer: "mycontainer",
		},
		{
			name:          "custom endpoint with path segments",
			location:      "custom.endpoint.com/account/container",
			wantEndpoint:  "custom.endpoint.com",
			wantAccount:   "account",
			wantContainer: "container",
		},
		{
			name:     "invalid simple location - missing container",
			location: "myaccount",
			wantErr:  true,
		},
		{
			name:     "invalid location - too many segments",
			location: "endpoint/account/container/extra",
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint, account, container, err := extractLocationAndContainer(tc.location)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantEndpoint, endpoint)
				assert.Equal(t, tc.wantAccount, account)
				assert.Equal(t, tc.wantContainer, container)
			}
		})
	}
}

func TestProviderID(t *testing.T) {
	assert.Equal(t, "AzureBlob", NewBackendProvider(nil).ID())
}
