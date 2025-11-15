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

package builtins

import (
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/types"
)

const helloBuiltinName = "chainloop.hello"

func RegisterHelloBuiltin() error {
	return Register(&BuiltinDef{
		Name: helloBuiltinName,
		Decl: &ast.Builtin{
			Name: helloBuiltinName,
			Decl: types.NewFunction(
				types.Args(
					types.Named("name", types.S), // Digest to fetch
				),
				types.Named("response", types.A), // Response as object
			),
		},
		Impl:          getHelloImpl,
		SecurityLevel: SecurityLevelRestrictive, // Always available
		Description:   "Discovers artifact graph data by calling the Referrer chainloop service",
	})
}

type helloResponse struct {
	Message string `json:"message"`
}

func getHelloImpl(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	if len(operands) < 1 {
		return errors.New("need one operand")
	}

	name, ok := operands[0].Value.(ast.String)
	if !ok {
		return errors.New("digest must be a string")
	}

	message := fmt.Sprintf("Hello, %s!", string(name))

	// call the iterator with the output value
	return iter(ast.NewTerm(ast.MustInterfaceToValue(helloResponse{message})))
}

func init() {
	if err := RegisterHelloBuiltin(); err != nil {
		panic(fmt.Sprintf("failed to register Hello builtin: %v", err))
	}
}
