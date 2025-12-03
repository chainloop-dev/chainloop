//
// Copyright 2025 The Chainloop Authors.
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

package _go

import (
	"encoding/json"
	"fmt"

	"github.com/extism/go-pdk"
)

// HTTPGet performs a GET request using Extism's built-in HTTP functionality.
// Only hostnames configured in the policy engine's AllowedHosts are accessible.
// Returns error if the hostname is not allowed or the request fails.
func HTTPGet(url string) ([]byte, error) {
	req := pdk.NewHTTPRequest(pdk.MethodGet, url)
	resp := req.Send()

	if resp.Status() != 200 {
		return nil, fmt.Errorf("HTTP request failed with status %d", resp.Status())
	}

	return resp.Body(), nil
}

// HTTPGetJSON performs a GET request and unmarshals the JSON response.
// Only hostnames configured in the policy engine's AllowedHosts are accessible.
func HTTPGetJSON(url string, target interface{}) error {
	body, err := HTTPGet(url)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return nil
}

// HTTPGetString performs a GET request and returns the response as a string.
// Only hostnames configured in the policy engine's AllowedHosts are accessible.
func HTTPGetString(url string) (string, error) {
	body, err := HTTPGet(url)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
