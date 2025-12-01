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

package main

import (
	"encoding/json"

	"github.com/extism/go-pdk"
)

//export Execute
func Execute() int32 {
	// Create a simple result with one violation for testing
	result := map[string]interface{}{
		"violations":   []string{"test violation"},
		"skip_reasons": []string{},
		"skipped":      false,
	}

	resultJSON, _ := json.Marshal(result)
	pdk.OutputString(string(resultJSON))
	return 0
}

func main() {}
