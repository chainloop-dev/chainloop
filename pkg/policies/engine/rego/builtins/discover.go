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

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/types"
	"google.golang.org/grpc"
)

const discoverBuiltinName = "chainloop.discover"

// RegisterDiscoverBuiltin is used to register chainloop's Discover endpoint as a builtin Rego function with signature:
//
//	chainloop.discover(digest, kind)
//
// For instance, to get the references for an CONTAINER_IMAGE material, and fail if any of them is an attestation with policy violations:
// ```
//
//	violations contains msg if {
//	  digest := sprintf("sha256:%s",[input.chainloop_metadata.digest.sha256])
//	  discovered := chainloop.discover(digest, "")
//
//	  some ref in discovered.references
//	  ref.kind == "ATTESTATION"
//	  ref.metadata.hasPolicyViolations == "true"
//
//	  msg:= sprintf("attestation with digest %s contains policy violations [name: %s, project: %s, org: %s]", [ref.digest, ref.metadata.name, ref.metadata.project, ref.metadata.organization])
//	}
//
// ```
func RegisterDiscoverBuiltin(conn *grpc.ClientConn) error {
	return Register(&ast.Builtin{
		Name:        discoverBuiltinName,
		Description: "Discovers artifact graph data by calling the Referrer chainloop service",
		Decl: types.NewFunction(
			types.Args(
				types.Named("digest", types.S).Description("digest of the artifact to discover"),
				types.Named("kind", types.S).Description("optional filter by kind to disambiguate"),
			),
			types.Named("response", types.A).Description("response object as in the `chainloop discover` CLI output"),
		),
		Nondeterministic: true,
	}, getDiscoverImpl(conn))
}

func getDiscoverImpl(conn *grpc.ClientConn) topdown.BuiltinFunc {
	return func(bctx topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
		if len(operands) < 1 {
			return errors.New("need at least one operand")
		}

		var digest, kind ast.String
		var ok bool

		// Extract digest
		digest, ok = operands[0].Value.(ast.String)
		if !ok {
			return errors.New("digest must be a string")
		}

		if len(operands) > 1 {
			// Extract kind
			kind, ok = operands[1].Value.(ast.String)
			if !ok {
				return errors.New("kind must be a string")
			}
		}

		// Call the service
		client := v1.NewReferrerServiceClient(conn)
		resp, err := client.DiscoverPrivate(bctx.Context, &v1.ReferrerServiceDiscoverPrivateRequest{
			Digest: string(digest), Kind: string(kind),
		})

		if err != nil {
			return fmt.Errorf("failed to call discover endpoint: %w", err)
		}

		// call the iterator with the output value
		return iter(ast.NewTerm(ast.MustInterfaceToValue(resp.Result)))
	}
}
