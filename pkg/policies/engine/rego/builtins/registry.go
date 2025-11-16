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

package builtins

import (
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

const (
	// NonRestrictiveBuiltin is used in builtin definition categories to mark a builtin as non-suitable for Chainloop's restrictive mode
	NonRestrictiveBuiltin = "non-restrictive"
)

// Register registers built-ins globally with OPA
// This should be called once during initialization
func Register(def *ast.Builtin, builtinFunc topdown.BuiltinFunc) error {
	// Register the built-in declaration with AST
	ast.RegisterBuiltin(def)

	// Register the implementation with topdown
	topdown.RegisterBuiltinFunc(def.Name, builtinFunc)
	return nil
}
