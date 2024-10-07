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
	"testing"

	"github.com/stretchr/testify/suite"
)

type providerTestSuite struct {
	suite.Suite

	registry *Registry
}

func TestProviderSuite(t *testing.T) {
	suite.Run(t, new(providerTestSuite))
}

func (s *providerTestSuite) SetupTest() {
	var err error
	s.registry, err = NewRegistry([]*NewRegistryConfig{
		{Name: "p1", Host: "https://p1host"},
		{Name: "p2", Host: "https://p2host"},
		{Name: "p3", Host: "https://p3host", Default: true},
	}...)
	s.Require().NoError(err)
}

func (s *providerTestSuite) TestDuplicateDefault() {
	_, err := NewRegistry([]*NewRegistryConfig{
		{Name: "p1", Host: "https://p1host"},
		{Name: "p2", Host: "https://p2host", Default: true},
		{Name: "p3", Host: "https://p3host", Default: true},
	}...)
	s.Error(err)
}

func (s *providerTestSuite) TestGetProvider() {
	cases := []struct {
		name         string
		providerName string
		expected     string
		expectedNil  bool
	}{{
		name: "returns the expected provider", providerName: "p1", expected: "p1",
	}, {
		name: "returns nil if none found", providerName: "p5", expectedNil: true,
	}}

	for _, c := range cases {
		s.Run(c.name, func() {
			p := s.registry.GetProvider(c.providerName)
			if c.expectedNil {
				s.Nil(p)
				return
			}
			s.Equal(c.expected, p.name)
		})
	}
}

func (s *providerTestSuite) TestHostAndUrlCompatibility() {
	cases := []struct {
		name        string
		host        string
		url         string
		expectedURL string
	}{
		{
			name:        "only host",
			host:        "http://myhost/v1/policies",
			expectedURL: "http://myhost/v1",
		},
		{
			name:        "only url",
			url:         "http://myhost/v1",
			expectedURL: "http://myhost/v1",
		},
		{
			name:        "both",
			host:        "http://myhost/v1/policies",
			url:         "http://myhost/v1",
			expectedURL: "http://myhost/v1",
		},
	}
	for _, c := range cases {
		s.Run(c.name, func() {
			r, err := NewRegistry(&NewRegistryConfig{
				Name: c.name,
				Host: c.host,
				URL:  c.url,
			})

			s.Require().NoError(err)
			s.Equal(c.expectedURL, r.providers[c.name].url)
		})
	}
}
