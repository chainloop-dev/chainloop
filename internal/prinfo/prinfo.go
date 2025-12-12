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

package prinfo

const (
	// EvidenceID is the identifier for the PR/MR info material type
	EvidenceID = "CHAINLOOP_PR_INFO"
	// EvidenceSchemaURL is the URL to the JSON schema for PR/MR info
	EvidenceSchemaURL = "https://schemas.chainloop.dev/prinfo/1.0/pr-info.schema.json"
)

// Data represents the data payload of the PR/MR info evidence
type Data struct {
	Platform     string `json:"platform" jsonschema:"required,enum=github,enum=gitlab,description=The CI/CD platform"`
	Type         string `json:"type" jsonschema:"required,enum=pull_request,enum=merge_request,description=The type of change request"`
	Number       string `json:"number" jsonschema:"required,description=The PR/MR number or identifier"`
	Title        string `json:"title,omitempty" jsonschema:"description=The PR/MR title"`
	Description  string `json:"description,omitempty" jsonschema:"description=The PR/MR description or body"`
	SourceBranch string `json:"source_branch,omitempty" jsonschema:"description=The source branch name"`
	TargetBranch string `json:"target_branch,omitempty" jsonschema:"description=The target branch name"`
	URL          string `json:"url" jsonschema:"required,format=uri,description=Direct URL to the PR/MR"`
	Author       string `json:"author,omitempty" jsonschema:"description=Username of the PR/MR author"`
}

// Evidence represents the complete evidence structure for PR/MR metadata
type Evidence struct {
	ID     string `json:"chainloop.material.evidence.id"`
	Schema string `json:"schema"`
	Data   Data   `json:"data"`
}

// NewEvidence creates a new Evidence instance
func NewEvidence(data Data) *Evidence {
	return &Evidence{
		ID:     EvidenceID,
		Schema: EvidenceSchemaURL,
		Data:   data,
	}
}
