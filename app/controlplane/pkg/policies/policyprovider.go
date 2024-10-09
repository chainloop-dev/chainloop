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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	policiesEndpoint = "policies"
	groupsEndpoint   = "groups"

	digestParam  = "digest"
	orgNameParam = "organization_name"
)

// PolicyProvider represents an external policy provider
type PolicyProvider struct {
	name, url string
	isDefault bool
}

type ProviderResponse struct {
	Data   map[string]any `json:"data"`
	Digest string         `json:"digest"`
}

type PolicyReference struct {
	URL    string
	Digest string
}

var ErrNotFound = fmt.Errorf("policy not found")

// Resolve calls the remote provider for retrieving a policy
func (p *PolicyProvider) Resolve(policyName, orgName, token string) (*schemaapi.Policy, *PolicyReference, error) {
	if policyName == "" || token == "" {
		return nil, nil, fmt.Errorf("both policyname and token are mandatory")
	}

	// the policy name might include a digest in the form of <name>@sha256:<digest>
	policyName, digest := policies.ExtractDigest(policyName)

	var policy schemaapi.Policy
	endpoint, err := url.JoinPath(p.url, policiesEndpoint, policyName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve policy: %w", err)
	}
	ref, err := p.queryProvider(endpoint, digest, orgName, token, &policy)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve policy: %w", err)
	}

	return &policy, ref, nil
}

// ResolveGroup calls remote provider for retrieving a policy group definition
func (p *PolicyProvider) ResolveGroup(groupName, orgName, token string) (*schemaapi.PolicyGroup, *PolicyReference, error) {
	if groupName == "" || token == "" {
		return nil, nil, fmt.Errorf("both policyname and token are mandatory")
	}

	// the policy name might include a digest in the form of <name>@sha256:<digest>
	policyName, digest := policies.ExtractDigest(groupName)

	var group schemaapi.PolicyGroup
	endpoint, err := url.JoinPath(p.url, groupsEndpoint, policyName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve group: %w", err)
	}
	ref, err := p.queryProvider(endpoint, digest, orgName, token, &group)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve group: %w", err)
	}

	return &group, ref, nil
}

func (p *PolicyProvider) queryProvider(path, digest, orgName, token string, out proto.Message) (*PolicyReference, error) {
	// craft the URL
	uri, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("error parsing policy provider URL: %w", err)
	}

	query := uri.Query()
	if digest != "" {
		query.Set(digestParam, digest)
	}

	if orgName != "" {
		query.Set(orgNameParam, orgName)
	}

	uri.RawQuery = query.Encode()

	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating policy request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing policy request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("expected status code 200 but got %d", resp.StatusCode)
	}

	resBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading policy response: %w", err)
	}

	// unmarshall response
	var response ProviderResponse
	if err := json.Unmarshal(resBytes, &response); err != nil {
		return nil, fmt.Errorf("error unmarshalling policy response: %w", err)
	}

	ref, err := p.resolveRef(path, response.Digest)
	if err != nil {
		return nil, fmt.Errorf("error resolving policy reference: %w", err)
	}

	// extract the policy payload from the query response
	jsonPolicy, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("error marshalling policy response: %w", err)
	}

	if err := protojson.Unmarshal(jsonPolicy, out); err != nil {
		return nil, fmt.Errorf("error unmarshalling policy response: %w", err)
	}

	return ref, nil
}

func (p *PolicyProvider) resolveRef(path, digest string) (*PolicyReference, error) {
	// Extract hostname from the policy provider URL
	uri, err := url.Parse(p.url)
	if err != nil {
		return nil, fmt.Errorf("error parsing policy provider URL: %w", err)
	}

	if uri.Host == "" {
		return nil, fmt.Errorf("invalid policy provider URL")
	}

	if path == "" || digest == "" {
		return nil, fmt.Errorf("both path and digest are mandatory")
	}

	return &PolicyReference{
		URL:    fmt.Sprintf("chainloop://%s/%s", uri.Host, path),
		Digest: digest,
	}, nil
}
