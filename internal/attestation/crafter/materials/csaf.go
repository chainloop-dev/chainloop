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

package materials

import (
	"fmt"

	"github.com/openvex/go-vex/pkg/csaf"
)

// CSAFValidator is an interface for validating CSAF documents
type CSAFValidator interface {
	ValidateDocument() error
}

// Common structs missing on "github.com/openvex/go-vex/pkg/csaf" implementation

type RevisionHistoryItem struct {
	Date    string `json:"date"`
	Number  int    `json:"string"`
	Summary string `json:"summary"`
}

type Tracking struct {
	csaf.Tracking
	RevisionHistory []RevisionHistoryItem `json:"revision_history"`
	Status          string                `json:"status"`
	Version         string                `json:"version"`
}

type CSAFDocument struct {
	csaf.CSAF
	Document struct {
		csaf.DocumentMetadata
		Tracking    Tracking    `json:"tracking"`
		Category    string      `json:"category"`
		CsafVersion string      `json:"csaf_version"`
		Notes       []csaf.Note `json:"notes"`
	} `json:"document"`
}

func (c *CSAFDocument) validateBaseDocument() error {
	// Validate required fields
	requiredFields := []string{
		c.Document.Category,
		c.Document.CsafVersion,
		c.Document.Publisher.Category,
		c.Document.Publisher.Name,
		c.Document.Publisher.Namespace,
		c.Document.Title,
		c.Document.Tracking.CurrentReleaseDate.String(),
		c.Document.Tracking.ID,
		c.Document.Tracking.InitialReleaseDate.String(),
		c.Document.Tracking.Status,
		c.Document.Tracking.Version,
	}

	for _, field := range requiredFields {
		if field == "" {
			return fmt.Errorf("required field is empty: %v", field)
		}
	}

	return nil
}

// SecurityIncidentResponse represents a CSAF document of type SecurityIncidentResponse
type SecurityIncidentResponse struct {
	CSAFDocument
}

// ValidateDocument validates the SecurityIncidentResponse document
func (s *SecurityIncidentResponse) ValidateDocument() error {
	return s.validateSecurityIncidentResponse()
}

// ValidateSecurityIncidentResponse validates the SecurityIncidentResponse document
func (s *SecurityIncidentResponse) validateSecurityIncidentResponse() error {
	if err := s.validateBaseDocument(); err != nil {
		return err
	}

	if len(s.Document.Notes) == 0 {
		return fmt.Errorf("notes are empty")
	}

	if len(s.Document.References) == 0 {
		return fmt.Errorf("references are empty")
	}

	return nil
}

// InformationalAdvisory represents a CSAF document of type InformationalAdvisory
type InformationalAdvisory struct {
	SecurityIncidentResponse
}

// ValidateDocument validates the InformationalAdvisory document
func (s *InformationalAdvisory) ValidateDocument() error {
	return s.validateSecurityIncidentResponse()
}

// SecurityAdvisory represents a CSAF document of type SecurityAdvisory
type SecurityAdvisory struct {
	CSAFDocument
}

// ValidateDocument validates the SecurityAdvisory document
func (s *SecurityAdvisory) ValidateDocument() error {
	return s.validateSecurityAdvisory()
}

// ValidateSecurityAdvisory validates the SecurityAdvisory document
func (s *SecurityAdvisory) validateSecurityAdvisory() error {
	if err := s.validateBaseDocument(); err != nil {
		return err
	}

	if len(s.ProductTree.ListProducts()) == 0 {
		return fmt.Errorf("notes are empty")
	}

	if len(s.Vulnerabilities) == 0 {
		return fmt.Errorf("references are empty")
	}

	return nil
}

// Vex represents a CSAF document of type VEX
type Vex struct {
	SecurityAdvisory
}

// ValidateDocument validates the Vex document
func (s *Vex) ValidateDocument() error {
	return s.validateSecurityAdvisory()
}
