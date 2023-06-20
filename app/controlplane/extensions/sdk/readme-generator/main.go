//
// Copyright 2023 The Chainloop Authors.
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

//go:generate go run main.go --dir ../../core

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	extensionsSDK "github.com/chainloop-dev/chainloop/app/controlplane/extensions"
	"github.com/go-kratos/kratos/v2/log"
)

const registrationInputHeader = "## Registration Input Schema"
const attachmentInputHeader = "## Attachment Input Schema"

// base path to the extensions directory
var extensionsDir string

func addSchemaToSection(src []byte, sectionHeader string, schema []byte) ([]byte, error) {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, schema, "", "  ")
	if err != nil {
		return nil, err
	}

	inputSection := sectionHeader + "\n\n```json\n" + prettyJSON.String() + "\n```"
	r := regexp.MustCompile(fmt.Sprintf("%s\n+```json\n+(.|\\s)*```", sectionHeader))
	// If the section already exists, replace it
	if r.Match(src) {
		return r.ReplaceAllLiteral(src, []byte(inputSection)), nil
	}

	// Append it
	return append(src, []byte("\n\n"+inputSection)...), nil
}

func mainE() error {
	l := log.NewStdLogger(os.Stdout)

	extensions, err := extensionsSDK.Load(l)
	if err != nil {
		return fmt.Errorf("failed to load extensions: %w", err)
	}

	for _, e := range extensions {
		// Find README file and extract its content
		file, err := os.OpenFile(filepath.Join(extensionsDir, e.Describe().ID, "v1", "README.md"), os.O_RDWR, 0644)
		if err != nil {
			return fmt.Errorf("failed to open README.md file: %w", err)
		}

		fileContent, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("failed to read README.md file: %w", err)
		}

		// Replace/Add registration input schema
		fileContent, err = addSchemaToSection(fileContent, registrationInputHeader, e.Describe().RegistrationJSONSchema)
		if err != nil {
			return fmt.Errorf("failed to add registration schema to README.md file: %w", err)
		}

		// Replace/Add attachment input schema
		fileContent, err = addSchemaToSection(fileContent, attachmentInputHeader, e.Describe().AttachmentJSONSchema)
		if err != nil {
			return fmt.Errorf("failed to add attachment schema to README.md file: %w", err)
		}

		// Write the new content in the file
		_, err = file.Seek(0, 0)
		if err != nil {
			return fmt.Errorf("failed to seek README.md file: %w", err)
		}

		_, err = file.Write(fileContent)
		if err != nil {
			fmt.Errorf("failed to write README.md file: %w", err)
		}

		_ = l.Log(log.LevelInfo, "msg", "README.md file updated", "extension", e.Describe().ID)
	}

	return nil
}

func main() {
	if err := mainE(); err != nil {
		panic(err)
	}
}

func init() {
	flag.StringVar(&extensionsDir, "dir", "", "base directory for extensions i.e ./core")
	flag.Parse()
}
