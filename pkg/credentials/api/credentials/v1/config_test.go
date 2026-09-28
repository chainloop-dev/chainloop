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

package v1_test

import (
	"testing"

	"buf.build/go/protovalidate"
	v1 "github.com/chainloop-dev/chainloop/pkg/credentials/api/credentials/v1"
	"github.com/stretchr/testify/assert"
)

// The Vault backend takes exactly one way to authenticate, enforced by the schema itself.
func TestVaultAuthIsExactlyOne(t *testing.T) {
	k8s := &v1.Credentials_Vault_KubernetesAuth{Role: "chainloop"}
	for name, tc := range map[string]struct {
		vault   *v1.Credentials_Vault
		wantErr bool
	}{
		"token":           {vault: &v1.Credentials_Vault{Address: "http://vault:8200", Token: "t"}},
		"kubernetes auth": {vault: &v1.Credentials_Vault{Address: "http://vault:8200", KubernetesAuth: k8s}},
		"neither":         {vault: &v1.Credentials_Vault{Address: "http://vault:8200"}, wantErr: true},
		"both":            {vault: &v1.Credentials_Vault{Address: "http://vault:8200", Token: "t", KubernetesAuth: k8s}, wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			err := protovalidate.Validate(tc.vault)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
