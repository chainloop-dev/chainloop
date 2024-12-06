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

package jacoco

type Counter struct {
	Type    string `xml:"type,attr" json:"type"`
	Missed  int    `xml:"missed,attr" json:"missed"`
	Covered int    `xml:"covered,attr" json:"covered"`
}

type Report struct {
	Counters []*Counter `xml:"counter" json:"counters"`
}
