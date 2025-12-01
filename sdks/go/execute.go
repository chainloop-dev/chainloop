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

// Run executes a policy function and returns the required int32 status code.
// This wrapper hides the WASM return value from users, making the API cleaner.
//
// Usage:
//
//	//export Execute
//	func Execute() int32 {
//	    return chainlooppolicy.Run(func() {
//	        var sbom SBOM
//	        if err := chainlooppolicy.GetMaterialJSON(&sbom); err != nil {
//	            chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
//	            return
//	        }
//
//	        result := chainlooppolicy.Success()
//	        // ... validation logic
//	        chainlooppolicy.OutputResult(result)
//	    })
//	}
//
// The function always returns 0, which indicates success to the WASM runtime.
func Run(policyFn func()) int32 {
	policyFn()
	return 0
}
