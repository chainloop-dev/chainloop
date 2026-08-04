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

package entities

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/grpcconn"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
)

type mockHeader map[string]string

func (h mockHeader) Get(key string) string { return h[key] }
func (h mockHeader) Set(key, value string) { h[key] = value }
func (h mockHeader) Add(key, value string) { h[key] = value }
func (h mockHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
func (h mockHeader) Values(key string) []string {
	if v, ok := h[key]; ok {
		return []string{v}
	}
	return nil
}

type mockTransport struct {
	header transport.Header
}

func (tr *mockTransport) Kind() transport.Kind            { return transport.KindGRPC }
func (tr *mockTransport) Endpoint() string                { return "" }
func (tr *mockTransport) Operation() string               { return "" }
func (tr *mockTransport) RequestHeader() transport.Header { return tr.header }
func (tr *mockTransport) ReplyHeader() transport.Header   { return tr.header }

func TestGetCLIVersionFromHeader(t *testing.T) {
	t.Run("returns the header value when present", func(t *testing.T) {
		ctx := transport.NewServerContext(context.Background(), &mockTransport{
			header: mockHeader{grpcconn.CLIVersionHeader: "v1.94.2-oss"},
		})
		assert.Equal(t, "v1.94.2-oss", GetCLIVersionFromHeader(ctx))
	})

	t.Run("returns empty when header is absent", func(t *testing.T) {
		ctx := transport.NewServerContext(context.Background(), &mockTransport{header: mockHeader{}})
		assert.Equal(t, "", GetCLIVersionFromHeader(ctx))
	})

	t.Run("returns empty when there is no transport in context", func(t *testing.T) {
		assert.Equal(t, "", GetCLIVersionFromHeader(context.Background()))
	})
}
