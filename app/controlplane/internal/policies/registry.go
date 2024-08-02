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

package policies

import (
	"strings"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
)

// Registry manages policy providers
type Registry struct {
	providers map[string]*PolicyProvider
}

func NewRegistry(conf ...*conf.PolicyProvider) *Registry {
	r := &Registry{providers: make(map[string]*PolicyProvider)}
	for _, p := range conf {
		r.providers[p.Name] = &PolicyProvider{
			name:      p.Name,
			host:      p.Host,
			isDefault: p.Default,
		}
	}
	return r
}

func (r *Registry) DefaultProvider() *PolicyProvider {
	for _, p := range r.providers {
		if p.isDefault {
			return p
		}
	}
	return nil
}

func (r *Registry) GetProvider(name string) *PolicyProvider {
	return r.providers[name]
}

// GetProviderFromReference finds a provider given a policy reference in the form of `provider://policy`.
// it returns the default provider if none found
func (r *Registry) GetProviderFromReference(ref string) *PolicyProvider {
	parts := strings.SplitAfterN(ref, "://", 2)
	if len(parts) == 2 {
		return r.providers[parts[0]]
	}
	return r.DefaultProvider()
}
