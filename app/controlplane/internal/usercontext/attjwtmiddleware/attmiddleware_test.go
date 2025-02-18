//
// Copyright 2024-2025 The Chainloop Authors.
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

package attjwtmiddleware_test

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
)

var emptyHandler = func(_ context.Context, _ interface{}) (interface{}, error) { return nil, nil }

type headerCarrier http.Header

func (hc headerCarrier) Get(key string) string { return http.Header(hc).Get(key) }

func (hc headerCarrier) Set(key string, value string) { http.Header(hc).Set(key, value) }

func (hc headerCarrier) Add(key string, value string) { http.Header(hc).Add(key, value) }

// Keys lists the keys stored in this carrier.
func (hc headerCarrier) Keys() []string {
	keys := make([]string, 0, len(hc))
	for k := range http.Header(hc) {
		keys = append(keys, k)
	}
	return keys
}

// Values returns a slice value associated with the passed key.
func (hc headerCarrier) Values(key string) []string {
	return http.Header(hc).Values(key)
}

func newTokenHeader(headerKey string, token string) *headerCarrier {
	header := &headerCarrier{}
	header.Set(headerKey, token)
	return header
}

type mockTransport struct {
	reqHeader transport.Header
}

func (tr *mockTransport) Kind() transport.Kind {
	return transport.KindGRPC
}

func (tr *mockTransport) Endpoint() string {
	return ""
}

func (tr *mockTransport) Operation() string {
	return ""
}

func (tr *mockTransport) RequestHeader() transport.Header {
	return tr.reqHeader
}

func (tr *mockTransport) ReplyHeader() transport.Header {
	return nil
}

const signingKey = "qwertyuiopasdfghjklzxcvbnm123456"

func TestAttestationAPITokenProvider(t *testing.T) {
	testCases := []struct {
		name           string
		tokenHeader    *headerCarrier
		wantErr        bool
		expectedError  string
		tokenProviders []attjwtmiddleware.JWTOption
	}{
		{
			name:           "invalid audience on robot account token",
			wantErr:        true,
			expectedError:  "unexpected token, invalid audience",
			tokenHeader:    newTokenHeader("Authorization", "Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE3MTUzMjY4NzIsImV4cCI6MTc0Njg2Mjg3MiwiYXVkIjoicmFuZG9tLmF1ZGllbmNlIiwic3ViIjoicmFuZG9tIHJhbmRvbSJ9.2tw4BWUAH_ISlk6rFTIQoDSxGEpUesmKae9pD3ck-_I"),
			tokenProviders: []attjwtmiddleware.JWTOption{attjwtmiddleware.NewRobotAccountProvider(signingKey)},
		},
		{
			name:           "invalid token",
			wantErr:        true,
			expectedError:  "signature is invalid",
			tokenHeader:    newTokenHeader("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.eyJSb2xlIjoiQWRtaW4iLCJJc3N1ZXIiOiJJc3N1ZXIiLCJVc2VybmFtZSI6IkphdmFJblVzZSIsImV4cCI6MTcxNTMzMjUwOSwiaWF0IjoxNzE1MzMyNTA5fQ.41X6FyZ5xo0ckpkOkQbe2wLpFZ4Emtb8aMy_-3ZFs6Y"),
			tokenProviders: []attjwtmiddleware.JWTOption{attjwtmiddleware.NewRobotAccountProvider(signingKey)},
		},
		{
			name:           "valid robot account token",
			tokenHeader:    newTokenHeader("Authorization", "Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE3MTUzMjY4NzIsImV4cCI6MTc0Njg2Mjg3MiwiYXVkIjoiYXR0ZXN0YXRpb25zLmNoYWlubG9vcCIsInN1YiI6InJhbmRvbSByYW5kb20ifQ._8EU10CtgVtfD6NEQCP4Uco7FAZ1mvqgsjuPkZNa0pc"),
			tokenProviders: []attjwtmiddleware.JWTOption{attjwtmiddleware.NewRobotAccountProvider(signingKey)},
		},
		{
			name:           "valid api token",
			tokenHeader:    newTokenHeader("Authorization", "Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE3MTUzMjY4NzIsImV4cCI6MTc0Njg2Mjg3MiwiYXVkIjoiYXBpLXRva2VuLWF1dGguY2hhaW5sb29wIiwic3ViIjoicmFuZG9tIHJhbmRvbSJ9.MHBtf4cKu2d8KDS5M5d-uKHYBbLEYOdhvacNFIB3MHI"),
			tokenProviders: []attjwtmiddleware.JWTOption{attjwtmiddleware.NewAPITokenProvider(signingKey)},
		},
		{
			name:        "token validates when multiple providers are set",
			tokenHeader: newTokenHeader("Authorization", "Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE3MTUzMjY4NzIsImV4cCI6MTc0Njg2Mjg3MiwiYXVkIjoiYXBpLXRva2VuLWF1dGguY2hhaW5sb29wIiwic3ViIjoicmFuZG9tIHJhbmRvbSJ9.MHBtf4cKu2d8KDS5M5d-uKHYBbLEYOdhvacNFIB3MHI"),
			tokenProviders: []attjwtmiddleware.JWTOption{
				attjwtmiddleware.NewRobotAccountProvider(signingKey),
				attjwtmiddleware.NewAPITokenProvider(signingKey),
			},
		},
	}

	logger := log.NewStdLogger(io.Discard)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := transport.NewServerContext(context.Background(), &mockTransport{reqHeader: tc.tokenHeader})

			m := attjwtmiddleware.WithJWTMulti(logger, tc.tokenProviders...)
			_, err := m(emptyHandler)(ctx, nil)

			if tc.wantErr {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
