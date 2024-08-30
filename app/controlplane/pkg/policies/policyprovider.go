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
	"google.golang.org/protobuf/encoding/protojson"
)

// PolicyProvider represents an external policy provider
type PolicyProvider struct {
	name, host string
	isDefault  bool
}

type ProviderResponse struct {
	Policy map[string]any `json:"policy"`
	Digest string         `json:"digest"`
}

type PolicyReference struct {
	URL    string
	Digest string
}

// Resolve calls the remote provider for retrieving a policy
func (p *PolicyProvider) Resolve(policyName string, token string) (*schemaapi.Policy, *PolicyReference, error) {
	if policyName == "" || token == "" {
		return nil, nil, fmt.Errorf("both policyname and token are mandatory")
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", p.host, policyName), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating policy request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("error executing policy request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("expected status code 200 but got %d", resp.StatusCode)
	}

	resBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading policy response: %w", err)
	}

	// unmarshall response
	var response ProviderResponse
	if err := json.Unmarshal(resBytes, &response); err != nil {
		return nil, nil, fmt.Errorf("error unmarshalling policy response: %w", err)
	}

	ref, err := p.resolveRef(policyName, response.Digest)
	if err != nil {
		return nil, nil, fmt.Errorf("error resolving policy reference: %w", err)
	}

	// extract the policy payload from the query response
	jsonPolicy, err := json.Marshal(response.Policy)
	if err != nil {
		return nil, nil, fmt.Errorf("error marshalling policy response: %w", err)
	}

	// unmarshall the payload to the known protobuf message for policies
	var res schemaapi.Policy
	if err := protojson.Unmarshal(jsonPolicy, &res); err != nil {
		return nil, nil, fmt.Errorf("error unmarshalling policy response: %w", err)
	}

	return &res, ref, nil
}

func (p *PolicyProvider) resolveRef(policyName, digest string) (*PolicyReference, error) {
	// Extract hostname from the policy provider URL
	uri, err := url.Parse(p.host)
	if err != nil {
		return nil, fmt.Errorf("error parsing policy provider URL: %w", err)
	}

	if uri.Host == "" {
		return nil, fmt.Errorf("invalid policy provider URL")
	}

	return &PolicyReference{
		URL:    fmt.Sprintf("chainloop://%s/%s", uri.Host, policyName),
		Digest: digest,
	}, nil
}
