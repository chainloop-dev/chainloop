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

package auditor

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/natsconn"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuditLogPublisher(t *testing.T) {
	tests := []struct {
		name string
		rc   *natsconn.ReloadableConnection
		opts []PublisherOption
		// nil publisher means disabled (no NATS configured)
		wantNil bool
	}{
		{
			name:    "nil connection disables the publisher",
			rc:      nil,
			wantNil: true,
		},
		{
			name:    "nil connection with options still disables the publisher",
			rc:      nil,
			opts:    []PublisherOption{WithoutStreamManagement()},
			wantNil: true,
		},
		{
			// publish-only mode skips stream creation/updates so it doesn't
			// need a live JetStream context at construction time
			name: "publish-only mode skips stream management",
			rc:   &natsconn.ReloadableConnection{},
			opts: []PublisherOption{WithoutStreamManagement()},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewAuditLogPublisher(context.Background(), tc.rc, log.DefaultLogger, tc.opts...)
			require.NoError(t, err)
			if tc.wantNil {
				assert.Nil(t, p)
				// nil publisher is a no-op
				assert.NoError(t, p.Publish(&EventPayload{}))
				return
			}

			assert.NotNil(t, p)
		})
	}
}
