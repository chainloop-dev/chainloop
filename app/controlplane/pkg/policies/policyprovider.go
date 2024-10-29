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
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing policy provider URL: %w", err)
	}
	providerDigest, err := p.queryProvider(url, digest, orgName, token, &policy)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve policy: %w", err)
	}

	return &policy, createRef(url, policyName, providerDigest, orgName), nil
}

// ResolveGroup calls remote provider for retrieving a policy group definition
func (p *PolicyProvider) ResolveGroup(groupName, orgName, token string) (*schemaapi.PolicyGroup, *PolicyReference, error) {
	if groupName == "" || token == "" {
		return nil, nil, fmt.Errorf("both policyname and token are mandatory")
	}

	// the policy name might include a digest in the form of <name>@sha256:<digest>
	groupName, digest := policies.ExtractDigest(groupName)

	var group schemaapi.PolicyGroup
	endpoint, err := url.JoinPath(p.url, groupsEndpoint, groupName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve group: %w", err)
	}
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing policy provider URL: %w", err)
	}
	providerDigest, err := p.queryProvider(url, digest, orgName, token, &group)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve group: %w", err)
	}

	return &group, createRef(url, groupName, providerDigest, orgName), nil
}

func (p *PolicyProvider) queryProvider(url *url.URL, digest, orgName, token string, out proto.Message) (string, error) {
	query := url.Query()
	if digest != "" {
		query.Set(digestParam, digest)
	}

	if orgName != "" {
		query.Set(orgNameParam, orgName)
	}

	url.RawQuery = query.Encode()

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return "", fmt.Errorf("error creating policy request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error executing policy request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return "", ErrNotFound
		}

		return "", fmt.Errorf("expected status code 200 but got %d", resp.StatusCode)
	}

	resBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading policy response: %w", err)
	}

	// unmarshall response
	var response ProviderResponse
	if err := json.Unmarshal(resBytes, &response); err != nil {
		return "", fmt.Errorf("error unmarshalling policy response: %w", err)
	}

	// extract the policy payload from the query response
	jsonPolicy, err := json.Marshal(response.Data)
	if err != nil {
		return "", fmt.Errorf("error marshalling policy response: %w", err)
	}

	if err := protojson.Unmarshal(jsonPolicy, out); err != nil {
		return "", fmt.Errorf("error unmarshalling policy response: %w", err)
	}

	return response.Digest, nil
}

func createRef(policyURL *url.URL, name, digest, orgName string) *PolicyReference {
	refURL := fmt.Sprintf("chainloop://%s/%s", policyURL.Host, name)
	if orgName != "" {
		refURL = fmt.Sprintf("%s?org=%s", refURL, orgName)
	}
	return &PolicyReference{
		URL:    refURL,
		Digest: digest,
	}
}
