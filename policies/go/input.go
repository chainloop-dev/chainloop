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

// GetArgs extracts policy arguments from Extism config.
// Returns the arguments map, or an empty map if no args are configured.
func GetArgs() (map[string]string, error) {
	argsJSON, exists := pdk.GetConfig("args")
	if !exists || argsJSON == "" {
		return map[string]string{}, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal args: %w", err)
	}

	// Convert to map[string]string
	result := make(map[string]string)
	for k, v := range args {
		if strVal, ok := v.(string); ok {
			result[k] = strVal
		} else {
			// Convert non-string values to string representation
			result[k] = fmt.Sprintf("%v", v)
		}
	}

	return result, nil
}

// GetArgString returns a single argument value by key.
// Returns empty string if the argument is not found.
func GetArgString(key string) (string, error) {
	args, err := GetArgs()
	if err != nil {
		return "", err
	}
	return args[key], nil
}

// GetArgStringDefault returns a single argument value by key with a default value.
// Returns the default value if the argument is not found or if there's an error.
func GetArgStringDefault(key, defaultValue string) string {
	args, err := GetArgs()
	if err != nil {
		return defaultValue
	}
	if val, ok := args[key]; ok && val != "" {
		return val
	}
	return defaultValue
}

// GetMaterialBytes returns the raw material bytes from the input.
// The material is passed as the main input to the WASM plugin.
func GetMaterialBytes() []byte {
	return pdk.Input()
}

// GetMaterialJSON unmarshals the material input as JSON into the target.
// Use this when the material is in JSON format.
func GetMaterialJSON(target interface{}) error {
	if err := pdk.InputJSON(target); err != nil {
		return fmt.Errorf("failed to unmarshal material: %w", err)
	}
	return nil
}

// GetMaterialString returns the material as a UTF-8 string.
// Use this for text-based materials.
func GetMaterialString() string {
	return string(pdk.Input())
}
