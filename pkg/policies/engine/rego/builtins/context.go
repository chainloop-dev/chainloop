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

package builtins

import "context"

// ProjectContext carries the project + version a policy is being evaluated against.
// It is attached to the per-evaluation context.Context by the rego engine so that
// chainloop.* built-ins can scope their requests (e.g. chainloop.findings) without
// requiring the rego author to pass project_name / project_version_name explicitly.
//
// Values may be empty when the engine has no project context (e.g. a local
// `chainloop policy develop eval` without --project flags). Built-ins must
// degrade gracefully in that case rather than erroring.
type ProjectContext struct {
	Name    string
	Version string
}

type projectContextKey struct{}

// WithProjectContext returns a derived context carrying the given project context.
func WithProjectContext(ctx context.Context, pc ProjectContext) context.Context {
	return context.WithValue(ctx, projectContextKey{}, pc)
}

// ProjectContextFromContext returns the project context attached to ctx, or the
// zero value if none was set. The bool reports whether a value was present.
func ProjectContextFromContext(ctx context.Context) (ProjectContext, bool) {
	pc, ok := ctx.Value(projectContextKey{}).(ProjectContext)
	return pc, ok
}
