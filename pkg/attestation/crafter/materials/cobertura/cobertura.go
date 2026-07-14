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

// Package cobertura provides the XML structs to parse Cobertura code coverage
// reports. See https://github.com/cobertura/cobertura and the coverage-04.dtd
// for the format definition.
package cobertura

import (
	"encoding/json"
	"encoding/xml"
	"math"
)

// Rate is a coverage ratio (line-rate, branch-rate, complexity). It serialises
// NaN/Inf as JSON null instead of erroring, because an empty-but-valid report —
// a service with no measurable lines, where coverage tools emit
// line-rate="NaN" (0/0) — must still project to valid JSON that the policy
// engine can evaluate. Without this, json.Marshal fails on NaN and the whole
// material becomes un-evaluable, which a policy would surface as a failure even
// though the report is legitimate. Policies should guard on lines-valid > 0
// before interpreting the rate so an empty report is treated as valid (no
// violations) rather than as 0% coverage.
type Rate float64

// MarshalJSON renders finite values as numbers and NaN/Inf as null.
func (r Rate) MarshalJSON() ([]byte, error) {
	f := float64(r)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return []byte("null"), nil
	}
	return json.Marshal(f)
}

// Coverage is the root <coverage> element of a Cobertura report.
type Coverage struct {
	XMLName         xml.Name  `xml:"coverage" json:"-"`
	LineRate        Rate      `xml:"line-rate,attr" json:"line-rate"`
	BranchRate      Rate      `xml:"branch-rate,attr" json:"branch-rate"`
	LinesCovered    int       `xml:"lines-covered,attr" json:"lines-covered"`
	LinesValid      int       `xml:"lines-valid,attr" json:"lines-valid"`
	BranchesCovered int       `xml:"branches-covered,attr" json:"branches-covered"`
	BranchesValid   int       `xml:"branches-valid,attr" json:"branches-valid"`
	Complexity      Rate      `xml:"complexity,attr" json:"complexity"`
	Version         string    `xml:"version,attr" json:"version"`
	Timestamp       int64     `xml:"timestamp,attr" json:"timestamp"`
	Sources         []string  `xml:"sources>source" json:"sources"`
	Packages        []Package `xml:"packages>package" json:"packages"`
}

// Package is a <package> element grouping classes.
type Package struct {
	Name       string  `xml:"name,attr" json:"name"`
	LineRate   Rate    `xml:"line-rate,attr" json:"line-rate"`
	BranchRate Rate    `xml:"branch-rate,attr" json:"branch-rate"`
	Complexity Rate    `xml:"complexity,attr" json:"complexity"`
	Classes    []Class `xml:"classes>class" json:"classes"`
}

// Class is a <class> element within a package.
type Class struct {
	Name       string   `xml:"name,attr" json:"name"`
	Filename   string   `xml:"filename,attr" json:"filename"`
	LineRate   Rate     `xml:"line-rate,attr" json:"line-rate"`
	BranchRate Rate     `xml:"branch-rate,attr" json:"branch-rate"`
	Complexity Rate     `xml:"complexity,attr" json:"complexity"`
	Methods    []Method `xml:"methods>method" json:"methods"`
	Lines      []Line   `xml:"lines>line" json:"lines"`
}

// Method is a <method> element within a class.
type Method struct {
	Name       string `xml:"name,attr" json:"name"`
	Signature  string `xml:"signature,attr" json:"signature"`
	LineRate   Rate   `xml:"line-rate,attr" json:"line-rate"`
	BranchRate Rate   `xml:"branch-rate,attr" json:"branch-rate"`
	Complexity Rate   `xml:"complexity,attr" json:"complexity"`
	Lines      []Line `xml:"lines>line" json:"lines"`
}

// Line is a <line> element describing coverage for a single source line.
type Line struct {
	Number int  `xml:"number,attr" json:"number"`
	Hits   int  `xml:"hits,attr" json:"hits"`
	Branch bool `xml:"branch,attr" json:"branch"`
	// ConditionCoverage is only present on branch lines (e.g. "50% (1/2)").
	ConditionCoverage string `xml:"condition-coverage,attr,omitempty" json:"condition-coverage,omitempty"`
}
