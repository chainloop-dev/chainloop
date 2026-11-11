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

package policies

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/errorsx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchRemoteErrorsRecordStack(t *testing.T) {
	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer notFound.Close()

	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	// Close the server so that the request fails in the transport.
	closed.Close()

	testCases := []struct {
		name       string
		url        string
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "non-2xx status returns a typed error",
			url:        notFound.URL,
			wantStatus: http.StatusNotFound,
			wantMsg:    "requesting remote policy: server returned HTTP 404",
		},
		{
			name:    "transport failure",
			url:     closed.URL,
			wantMsg: "requesting remote policy: ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fetchRemote(nil, tc.url)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
			assert.True(t, errorsx.HasStack(err))

			var statusErr *RemoteStatusError
			if tc.wantStatus == 0 {
				assert.NotErrorAs(t, err, &statusErr)
				return
			}
			require.ErrorAs(t, err, &statusErr)
			assert.Equal(t, tc.wantStatus, statusErr.StatusCode)
		})
	}
}
