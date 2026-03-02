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

package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "long path is masked with last 4 visible",
			raw:  "https://example.com/some/long/path/with/many/segments/secret-token",
			want: "https://example.com****oken",
		},
		{
			name: "short path is not masked",
			raw:  "https://example.com/sho",
			want: "https://example.com/sho",
		},
		{
			name: "path at threshold is not masked",
			raw:  "https://example.com/abc",
			want: "https://example.com/abc",
		},
		{
			name: "path above threshold is masked",
			raw:  "https://example.com/abcde",
			want: "https://example.com****bcde",
		},
		{
			name: "empty string returns empty",
			raw:  "",
			want: "",
		},
		{
			name: "missing host returns empty",
			raw:  "/just/a/path",
			want: "",
		},
		{
			name: "webhook URL with port",
			raw:  "https://prod-00.westus.logic.azure.com:443/workflows/1234567890abcdef/triggers/manual/paths/invoke",
			want: "https://prod-00.westus.logic.azure.com:443****voke",
		},
		{
			name: "no path",
			raw:  "https://example.com",
			want: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskURL(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}
