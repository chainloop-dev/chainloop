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
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
)

type NewRegistryConfig struct {
	Name    string
	Host    string
	URL     string
	Default bool
}

// Registry manages policy providers
type Registry struct {
	providers map[string]*PolicyProvider
}

func NewRegistry(conf ...*NewRegistryConfig) (*Registry, error) {
	r := &Registry{providers: make(map[string]*PolicyProvider)}
	var hasDefault bool

	for _, p := range conf {
		if hasDefault && p.Default {
			return nil, fmt.Errorf("duplicate default policy")
		}
		hasDefault = hasDefault || p.Default
		// For backwards compatibility, if host is provided, we get the URI from it, by extracting "/policies" suffix
		if p.URL == "" && p.Host != "" {
			p.URL, _ = strings.CutSuffix(p.Host, fmt.Sprintf("/%s", policiesEndpoint))
		}
		r.providers[p.Name] = &PolicyProvider{
			name:      p.Name,
			url:       p.URL,
			isDefault: p.Default,
		}
	}
	return r, nil
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
	if name == "" {
		return r.DefaultProvider()
	}

	return r.providers[name]
}

func (r *Registry) GetProviderNames() []string {
	return maps.Keys(r.providers)
}
