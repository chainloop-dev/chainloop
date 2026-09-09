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

package config

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// SlugifyDNS1123 turns a free-form project name into a DNS-1123 label, the form
// the control plane accepts. It lowercases, replaces every run of characters
// that are not lowercase alphanumerics with a single dash, and trims the dashes
// off both ends. Names longer than the limit are cut, and cutting never leaves a
// trailing dash behind.
//
// It returns an empty string for input that has nothing usable in it, e.g.
// "!!!". Callers are expected to reject that rather than pass it on;
// ValidateDNS1123Label produces the message for it.
//
// materials.SanitizeMaterialName does the same to material names, and the two
// agree on everything but the length cap, which it leaves to its own caller.
// They are kept apart on purpose: importing it here would put the whole
// material crafter, some 377 packages, behind this one.
func SlugifyDNS1123(name string) string {
	var b strings.Builder
	b.Grow(len(name))

	// Collapse separator runs as we go, so "a  b" and "a-b" produce the same slug.
	pendingSeparator := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingSeparator && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingSeparator = false
			b.WriteRune(r)
			continue
		}

		pendingSeparator = true
	}

	slug := b.String()
	if len(slug) > validation.DNS1123LabelMaxLength {
		slug = strings.TrimRight(slug[:validation.DNS1123LabelMaxLength], "-")
	}

	return slug
}

// The kinds of thing ValidateDNS1123Label names in what it reports. Projects
// and organizations are named by the same rule, and the caller says which one
// it is asking about so the message names it.
const (
	ProjectSubject      = "project"
	OrganizationSubject = "organization"
)

// ValidateDNS1123Label reports whether name is one the control plane will
// accept for a subject of that kind, using the same rule the server applies at
// creation time so a bad name is caught at the prompt rather than by a failing
// API call.
func ValidateDNS1123Label(subject, name string) error {
	if name == "" {
		return fmt.Errorf("the %s name cannot be empty", subject)
	}

	if len(name) > validation.DNS1123LabelMaxLength {
		return fmt.Errorf("the %s name is %d characters long, the maximum is %d", subject, len(name), validation.DNS1123LabelMaxLength)
	}

	if errs := validation.IsDNS1123Label(name); len(errs) > 0 {
		return fmt.Errorf("the %s name must contain only lowercase letters, numbers and dashes, and start and end with a letter or a number", subject)
	}

	return nil
}
