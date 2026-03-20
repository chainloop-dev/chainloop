//
// Copyright 2025-2026 The Chainloop Authors.
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
	EvidenceSchemaURL = "https://schemas.chainloop.dev/prinfo/1.2/pr-info.schema.json"
)

// Reviewer represents a reviewer of the PR/MR
type Reviewer struct {
	Login        string `json:"login" jsonschema:"required,description=Username of the reviewer"`
	Type         string `json:"type" jsonschema:"required,enum=User,enum=Bot,enum=unknown,description=Account type of the reviewer"`
	Requested    bool   `json:"requested" jsonschema:"required,description=Whether the reviewer was explicitly requested to review"`
	ReviewStatus string `json:"review_status,omitempty" jsonschema:"enum=APPROVED,enum=CHANGES_REQUESTED,enum=COMMENTED,enum=DISMISSED,enum=PENDING,description=The reviewer's current review state if they have submitted a review"`
}

// Data represents the data payload of the PR/MR info evidence
type Data struct {
	Platform     string     `json:"platform" jsonschema:"required,enum=github,enum=gitlab,description=The CI/CD platform"`
	Type         string     `json:"type" jsonschema:"required,enum=pull_request,enum=merge_request,description=The type of change request"`
	Number       string     `json:"number" jsonschema:"required,description=The PR/MR number or identifier"`
	Title        string     `json:"title" jsonschema:"description=The PR/MR title"`
	Description  string     `json:"description" jsonschema:"description=The PR/MR description or body"`
	SourceBranch string     `json:"source_branch" jsonschema:"description=The source branch name"`
	TargetBranch string     `json:"target_branch" jsonschema:"description=The target branch name"`
	URL          string     `json:"url" jsonschema:"required,format=uri,description=Direct URL to the PR/MR"`
	Author       string     `json:"author" jsonschema:"description=Username of the PR/MR author"`
	Reviewers    []Reviewer `json:"reviewers,omitempty" jsonschema:"description=List of reviewers who reviewed or were requested to review"`
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
