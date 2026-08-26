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

// Package pitest provides the XML structs to parse PIT mutation testing
// reports (mutations.xml). See https://pitest.org/ and PIT's
// XMLReportListener for the format definition.
package pitest

import "encoding/xml"

// Report is the root <mutations> element of a PIT report.
type Report struct {
	XMLName xml.Name `xml:"mutations" json:"-"`
	// Partial mirrors PIT's partial root attribute: a partial report covers
	// only the mutations PIT analyzed before the run was cut short.
	Partial   bool       `xml:"partial,attr" json:"partial"`
	Mutations []Mutation `xml:"mutation" json:"mutations"`
}

// Mutation is a single <mutation> record. Status is preserved verbatim
// (KILLED, SURVIVED, NO_COVERAGE, ...) rather than validated against a closed
// enum or derived from Detected, so NO_COVERAGE stays distinguishable from
// SURVIVED and the projection stays forward-compatible with new PIT statuses.
type Mutation struct {
	Detected          bool   `xml:"detected,attr" json:"detected"`
	Status            string `xml:"status,attr" json:"status"`
	NumberOfTestsRun  int    `xml:"numberOfTestsRun,attr" json:"numberOfTestsRun"`
	SourceFile        string `xml:"sourceFile" json:"sourceFile"`
	MutatedClass      string `xml:"mutatedClass" json:"mutatedClass"`
	MutatedMethod     string `xml:"mutatedMethod" json:"mutatedMethod"`
	MethodDescription string `xml:"methodDescription" json:"methodDescription"`
	LineNumber        int    `xml:"lineNumber" json:"lineNumber"`
	Mutator           string `xml:"mutator" json:"mutator"`
	Indexes           []int  `xml:"indexes>index" json:"indexes"`
	Blocks            []int  `xml:"blocks>block" json:"blocks"`
	// Standard reports emit KillingTest; reports generated with
	// fullMutationMatrix emit KillingTests, SucceedingTests and CoveringTests
	// instead, each a '|'-delimited list of test names kept unsplit. Only
	// these four fields are omitempty so both report shapes keep their native
	// representation without manufacturing absent fields.
	KillingTest     string `xml:"killingTest" json:"killingTest,omitempty"`
	KillingTests    string `xml:"killingTests" json:"killingTests,omitempty"`
	SucceedingTests string `xml:"succeedingTests" json:"succeedingTests,omitempty"`
	CoveringTests   string `xml:"coveringTests" json:"coveringTests,omitempty"`
	Description     string `xml:"description" json:"description"`
}
