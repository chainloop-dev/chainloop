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

package authz

// MembershipType represents a polymorphic membership subject (user or group)
type MembershipType string

// ResourceType represent a membership resource (organizations, projects)
type ResourceType string

const (
	MembershipTypeUser  MembershipType = "user"
	MembershipTypeGroup MembershipType = "group"

	ResourceTypeOrganization ResourceType = "organization"
	ResourceTypeProject      ResourceType = "project"
	ResourceTypeProduct      ResourceType = "product"
	ResourceTypeGroup        ResourceType = "group"
)

// Values implement https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (MembershipType) Values() (values []string) {
	values = append(values,
		string(MembershipTypeUser),
		string(MembershipTypeGroup),
	)

	return
}

// Values implement https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (ResourceType) Values() (values []string) {
	values = append(values,
		string(ResourceTypeOrganization),
		string(ResourceTypeProject),
		string(ResourceTypeGroup),
		string(ResourceTypeProduct),
	)

	return
}
