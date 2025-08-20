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

package action

import (
	"github.com/chainloop-dev/chainloop/app/cli/internal/policydevel"
)

type PolicyEvalOpts struct {
	MaterialPath     string
	Kind             string
	Annotations      map[string]string
	PolicyPath       string
	Inputs           map[string]string
	AllowedHostnames []string
	Debug            bool
	EnablePrint      bool
}

type PolicyEval struct {
	*ActionsOpts
	opts *PolicyEvalOpts
}

func NewPolicyEval(opts *PolicyEvalOpts, actionOpts *ActionsOpts) (*PolicyEval, error) {
	return &PolicyEval{
		ActionsOpts: actionOpts,
		opts:        opts,
	}, nil
}

func (action *PolicyEval) Run() (*policydevel.EvalSummary, error) {
	evalOpts := &policydevel.EvalOptions{
		PolicyPath:       action.opts.PolicyPath,
		MaterialKind:     action.opts.Kind,
		Annotations:      action.opts.Annotations,
		MaterialPath:     action.opts.MaterialPath,
		Inputs:           action.opts.Inputs,
		AllowedHostnames: action.opts.AllowedHostnames,
		Debug:            action.opts.Debug,
		EnablePrint:      action.opts.EnablePrint,
	}

	// Evaluate policy
	resp, err := policydevel.Evaluate(evalOpts, action.Logger)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
