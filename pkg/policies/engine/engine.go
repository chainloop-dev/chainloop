//
// Copyright 2024 The Chainloop Authors.
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

package engine

import (
	"context"
)

type PolicyEngine interface {
	Verify(ctx context.Context, policy *Policy, input []byte) ([]*PolicyViolation, error)
}

type PolicyViolation struct {
	Subject, Violation string
}

// Policy represents a loaded policy in any of the supported technologies.
type Policy struct {
	//
	Module []byte `json:"module"`
	Name   string `json:"name"`
}
