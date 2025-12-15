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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/chainloop-dev/chainloop/internal/prinfo"
)

//go:generate go run ./generate.go --output-dir ../../../internal/schemavalidators/internal_schemas/prinfo --version 1.0
func main() {
	var outputDir string
	var version string

	flag.StringVar(&outputDir, "output-dir", "../../../internal/schemavalidators/internal_schemas/prinfo", "Directory to output the schema files")
	flag.StringVar(&version, "version", "1.0", "Schema version")
	flag.Parse()

	generator := prinfo.NewGenerator()

	fmt.Printf("Generating JSON schema for PR/MR Info\n")
	sch := generator.GeneratePRInfoSchema(version)
	if err := generator.Save(sch, outputDir, version); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("JSON schema successfully generated at %s/pr-info-%s.schema.json\n", outputDir, version)
}
