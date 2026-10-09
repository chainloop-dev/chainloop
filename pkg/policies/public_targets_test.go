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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"

	v12 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/netguard"
)

// countingServer serves the given files and counts the requests that reach it.
// httptest listens on the loopback interface, so it stands in for any
// destination that is not publicly routable.
func (s *testSuite) countingServer(newServer func(http.Handler) *httptest.Server, files map[string]string) (*httptest.Server, *atomic.Int32) {
	var requests atomic.Int32
	server := newServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		content, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(content))
	}))
	s.T().Cleanup(server.Close)

	return server, &requests
}

func (s *testSuite) readTestdata(path string) string {
	content, err := os.ReadFile(path)
	s.Require().NoError(err)
	return string(content)
}

func (s *testSuite) TestPublicTargetsOnlyPolicyHTTPRequests() {
	server, requests := s.countingServer(httptest.NewServer, nil)
	serverURL, err := url.Parse(server.URL)
	s.Require().NoError(err)

	policy := &v12.Policy{
		ApiVersion: "chainloop.dev/v1",
		Kind:       "Policy",
		Metadata:   &v12.Metadata{Name: "http-send"},
		Spec: &v12.PolicySpec{Policies: []*v12.PolicySpecV2{{
			Kind: v12.CraftingSchema_Material_ATTESTATION,
			Source: &v12.PolicySpecV2_Embedded{Embedded: fmt.Sprintf(`package main

import rego.v1

result := {"skipped": false, "violations": violations}

violations contains "request sent" if {
	http.send({"method": "GET", "url": %q})
}
`, server.URL)},
		}}},
	}
	policies := &v12.Policies{
		Attestation: []*v12.PolicyAttachment{{Policy: &v12.PolicyAttachment_Embedded{Embedded: policy}}},
	}

	cases := []struct {
		name       string
		publicOnly bool
	}{
		{name: "http.send reaches a private target by default"},
		{name: "http.send to a private target is blocked", publicOnly: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			// OPA matches the allowed host without its port
			opts := []PolicyVerifierOption{WithAllowedHostnames(serverURL.Hostname())}
			if tc.publicOnly {
				opts = append(opts, WithPublicTargetsOnly())
			}
			requests.Store(0)

			verifier := NewPolicyVerifier(policies, nil, &s.logger, opts...)
			_, err := verifier.VerifyStatement(context.TODO(), loadStatement("testdata/statement.json", &s.Suite))

			if tc.publicOnly {
				s.ErrorContains(err, netguard.ErrBlockedTarget.Error())
				s.Zero(requests.Load(), "a private target must not be reached")
				return
			}

			s.NoError(err)
			s.NotZero(requests.Load())
		})
	}
}

func (s *testSuite) TestPublicTargetsOnlyRemoteLoaders() {
	server, requests := s.countingServer(httptest.NewServer, map[string]string{
		"/policy.yaml": s.readTestdata("testdata/workflow_embedded.yaml"),
		"/group.yaml":  s.readTestdata("testdata/policy_group.yaml"),
	})

	cases := []struct {
		name       string
		publicOnly bool
	}{
		{name: "loaders reach a private target by default"},
		{name: "loading from a private target is blocked", publicOnly: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			var opts []PolicyVerifierOption
			if tc.publicOnly {
				opts = append(opts, WithPublicTargetsOnly())
			}
			pgv := NewPolicyGroupVerifier(nil, nil, nil, &s.logger, opts...)

			loaders := map[string]func() error{
				"policy spec": func() error {
					_, _, err := pgv.loadPolicySpec(context.TODO(), &v12.PolicyAttachment{
						Policy: &v12.PolicyAttachment_Ref{Ref: server.URL + "/policy.yaml"},
					})
					return err
				},
				"policy group": func() error {
					_, _, err := LoadPolicyGroup(context.TODO(), &v12.PolicyGroupAttachment{Ref: server.URL + "/group.yaml"},
						&LoadPolicyGroupOptions{Logger: &s.logger, HTTPClient: pgv.httpClient})
					return err
				},
			}

			for name, load := range loaders {
				requests.Store(0)
				err := load()

				if tc.publicOnly {
					s.ErrorIs(err, netguard.ErrBlockedTarget, name)
					s.Zero(requests.Load(), "%s: a private target must not be reached", name)
					continue
				}

				s.NoError(err, name)
				s.Equal(int32(1), requests.Load(), name)
			}
		})
	}
}

func (s *testSuite) TestPublicTargetsOnlyScriptFetch() {
	server, requests := s.countingServer(httptest.NewTLSServer, map[string]string{
		"/policy.rego": "package main",
	})

	cases := []struct {
		name       string
		publicOnly bool
	}{
		{name: "script fetch reaches a private target by default"},
		{name: "script fetch from a private target is blocked", publicOnly: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			// The test server's own client trusts its certificate. The guarded
			// client is taken from a verifier to cover how it is wired there.
			client := server.Client()
			if tc.publicOnly {
				client = NewPolicyVerifier(nil, nil, &s.logger, WithPublicTargetsOnly()).httpClient
			}

			requests.Store(0)
			content, err := loadScriptContent(server.URL+"/policy.rego", "", client)

			if tc.publicOnly {
				s.ErrorIs(err, netguard.ErrBlockedTarget)
				s.Zero(requests.Load(), "a private target must not be reached")
				return
			}

			s.NoError(err)
			s.Equal("package main", string(content))
		})
	}
}
