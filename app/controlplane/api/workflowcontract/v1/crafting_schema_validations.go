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

package v1

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// CraftingMaterialInValidationOrder all type of CraftingMaterial that are available for automatic
// detection. The order of the list is important as it defines the order of the
// detection process. Normally from most common one to the least common one and weaker validation method.
var CraftingMaterialInValidationOrder = []CraftingSchema_Material_MaterialType{
	CraftingSchema_Material_OPENVEX,
	CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
	CraftingSchema_Material_SBOM_SPDX_JSON,
	CraftingSchema_Material_CSAF_VEX,
	CraftingSchema_Material_CSAF_INFORMATIONAL_ADVISORY,
	CraftingSchema_Material_CSAF_SECURITY_ADVISORY,
	CraftingSchema_Material_CSAF_SECURITY_INCIDENT_RESPONSE,
	CraftingSchema_Material_GITLAB_SECURITY_REPORT,
	CraftingSchema_Material_JUNIT_XML,
	CraftingSchema_Material_JACOCO_XML,
	CraftingSchema_Material_HELM_CHART,
	CraftingSchema_Material_SARIF,
	CraftingSchema_Material_BLACKDUCK_SCA_JSON,
	CraftingSchema_Material_TWISTCLI_SCAN_JSON,
	CraftingSchema_Material_ZAP_DAST_ZIP,
	CraftingSchema_Material_SLSA_PROVENANCE,
	CraftingSchema_Material_ATTESTATION,
	CraftingSchema_Material_CONTAINER_IMAGE,
	CraftingSchema_Material_ARTIFACT,
	CraftingSchema_Material_STRING,
}

// ListAvailableMaterialKind returns a list of available material kinds
func ListAvailableMaterialKind() []string {
	var res []string
	for k := range CraftingSchema_Material_MaterialType_value {
		if k != "MATERIAL_TYPE_UNSPECIFIED" {
			res = append(res, strings.Replace(k, "MATERIAL_TYPE_", "", 1))
		}
	}

	return res
}

// Custom validations

// ValidateUniqueMaterialName validates that only one material definition
// with the same ID is present in the schema
func (schema *CraftingSchema) ValidateUniqueMaterialName() error {
	materialNames := make(map[string]bool)
	for _, m := range schema.Materials {
		if _, found := materialNames[m.Name]; found {
			return fmt.Errorf("material with name=%s is duplicated", m.Name)
		}

		materialNames[m.Name] = true
	}

	return nil
}

func (schema *CraftingSchema) ValidatePolicyAttachments() error {
	attachments := append(schema.GetPolicies().GetAttestation(), schema.GetPolicies().GetMaterials()...)

	for _, att := range attachments {
		// Validate refs.
		if att.GetRef() != "" {
			if err := ValidatePolicyAttachmentRef(att.GetRef()); err != nil {
				return fmt.Errorf("invalid reference %q: %w", att.GetRef(), err)
			}
		}
	}

	return nil
}

func ValidatePolicyAttachmentRef(ref string) error {
	if ref == "" {
		return fmt.Errorf("reference is empty")
	}

	// validate the optional digest format
	parts := strings.SplitN(ref, "@", 2)
	if len(parts) > 1 {
		rawDigest := parts[1]
		if _, err := cr_v1.NewHash(rawDigest); err != nil {
			return fmt.Errorf("invalid digest, want policy-ref@sha256:[hex]: %w", err)
		}

		// remove it @sha256: suffix
		ref = strings.TrimSuffix(ref, fmt.Sprintf("@%s", parts[1]))
	}

	var scheme, refValue string
	refParts := strings.SplitN(ref, "://", 2)
	if len(refParts) == 2 {
		scheme = refParts[0]
		refValue = refParts[1]
	} else {
		refValue = refParts[0]
	}

	switch scheme {
	case "file":
		u, err := url.Parse(ref)
		if err != nil {
			return fmt.Errorf("invalid reference: %w", err)
		}
		// file URLs like file://my-policy.yaml are parsed as only Host with empty path
		path := u.Path
		if path == "" {
			path = u.Host
		}
		if path == "" {
			return fmt.Errorf("invalid file reference %q", u.String())
		}
		if filepath.Ext(path) == "" {
			return fmt.Errorf("missing extension")
		}
	case "http", "https":
		u, err := url.Parse(ref)
		if err != nil {
			return fmt.Errorf("invalid reference: %w", err)
		}
		if u.Path == "" {
			return fmt.Errorf("path is empty")
		} else if filepath.Ext(u.Path) == "" {
			return fmt.Errorf("missing extension")
		}
	case "chainloop", "": // empty scheme means chainloop
		// split the path into provider name and policy name
		// chainloop://provider-name:org_name/policy-name
		// chainloop://policy-name
		// NOTE that the provider name is optional
		parts := strings.SplitN(refValue, ":", 2)
		// This will be used when the policy is a chainloop policy
		// provided by a remote policy provider
		var providerName, policyName, orgName string

		if len(parts) == 1 {
			policyName = parts[0]
		} else {
			providerName = parts[0]
			policyName = parts[1]
		}
		scoped := strings.SplitN(policyName, "/", 2)
		if len(scoped) == 2 {
			orgName = scoped[0]
			policyName = scoped[1]
		}

		if err := validateIsDNS1123(policyName); err != nil {
			return fmt.Errorf("invalid policy name: %w", err)
		}

		if providerName != "" {
			if err := validateIsDNS1123(providerName); err != nil {
				return fmt.Errorf("invalid provider name: %w", err)
			}
		}
		if orgName != "" {
			if err := validateIsDNS1123(orgName); err != nil {
				return fmt.Errorf("invalid organization name: %w", err)
			}
		}
	default:
		return fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return nil
}

func validateIsDNS1123(name string) error {
	// The same validation done by Kubernetes for their namespace name
	// https://github.com/kubernetes/apimachinery/blob/fa98d6eaedb4caccd69fc07d90bbb6a1e551f00f/pkg/api/validation/generic.go#L63
	err := validation.IsDNS1123Label(name)
	if len(err) > 0 {
		errMsg := ""
		for _, e := range err {
			errMsg += fmt.Sprintf("%q: %s\n", name, e)
		}

		return errors.New(errMsg)
	}

	return nil
}
