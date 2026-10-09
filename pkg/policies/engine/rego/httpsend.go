//
// Copyright 2026 The Chainloop Authors.
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

package rego

import (
	"context"
	"fmt"
	"net/url"
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// allowedHTTPSendSchemes are the URL schemes http.send may use. Other schemes,
// such as unix, make the evaluating process connect to local resources.
var allowedHTTPSendSchemes = []string{"http", "https"}

var httpSendURLKey = ast.StringTerm("url")

// restrictedHTTPSendOptions are http.send request options that make the
// evaluating process read local files or environment variables.
var restrictedHTTPSendOptions = []*ast.Term{
	ast.StringTerm("tls_ca_cert_file"),
	ast.StringTerm("tls_ca_cert_env_variable"),
	ast.StringTerm("tls_client_cert_file"),
	ast.StringTerm("tls_client_cert_env_variable"),
	ast.StringTerm("tls_client_key_file"),
	ast.StringTerm("tls_client_key_env_variable"),
}

type permissiveModeCtxKey struct{}

// withPermissiveMode marks the evaluation context so that http.send accepts
// the restricted options. Any evaluation without this mark rejects them.
func withPermissiveMode(ctx context.Context) context.Context {
	return context.WithValue(ctx, permissiveModeCtxKey{}, true)
}

func isPermissiveMode(ctx context.Context) bool {
	permissive, _ := ctx.Value(permissiveModeCtxKey{}).(bool)
	return permissive
}

// OPA resolves built-ins from a global registry, so http.send can only be
// restricted by replacing its implementation there.
func init() {
	httpSend := topdown.GetBuiltin(ast.HTTPSend.Name)
	topdown.RegisterBuiltinFunc(ast.HTTPSend.Name, func(bctx topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
		if !isPermissiveMode(bctx.Context) {
			if err := validateHTTPSendRequest(operands); err != nil {
				return err
			}
		}
		return httpSend(bctx, operands, iter)
	})
}

func validateHTTPSendRequest(operands []*ast.Term) error {
	req, ok := operands[0].Value.(ast.Object)
	if !ok {
		return nil
	}
	if urlTerm := req.Get(httpSendURLKey); urlTerm != nil {
		rawURL, _ := urlTerm.Value.(ast.String)
		if u, err := url.Parse(string(rawURL)); err == nil && !slices.Contains(allowedHTTPSendSchemes, u.Scheme) {
			return fmt.Errorf("url scheme %q is not allowed", u.Scheme)
		}
	}
	for _, opt := range restrictedHTTPSendOptions {
		if req.Get(opt) != nil {
			return fmt.Errorf("request option %v is not allowed", opt)
		}
	}
	return nil
}
