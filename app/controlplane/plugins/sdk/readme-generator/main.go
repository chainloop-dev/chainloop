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
	"sort"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
)

const registrationInputHeader = "## Registration Input Schema"
const attachmentInputHeader = "## Attachment Input Schema"

// base path to the plugins directory
var pluginsDir string

// Enhance README.md files for the registrations with the registration and attachment input schemas
func mainE() error {
	l := log.NewStdLogger(os.Stdout)

	plugins, err := plugins.Load(l)
	if err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	for _, e := range plugins {
		// Find README file and extract its content
		file, err := os.OpenFile(filepath.Join(pluginsDir, e.Describe().ID, "v1", "README.md"), os.O_RDWR, 0644)
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
			return fmt.Errorf("failed to write README.md file: %w", err)
		}

		_ = l.Log(log.LevelInfo, "msg", "README.md file updated", "plugin", e.Describe().ID)
	}

	return nil
}

func main() {
	if err := mainE(); err != nil {
		panic(err)
	}
}

func init() {
	flag.StringVar(&pluginsDir, "dir", "", "base directory for plugins i.e ./core")
	flag.Parse()
}

func addSchemaToSection(src []byte, sectionHeader string, schema []byte) ([]byte, error) {
	var jsonSchema bytes.Buffer
	err := json.Indent(&jsonSchema, schema, "", "  ")
	if err != nil {
		return nil, err
	}

	propertiesTable, err := renderSchemaTable(schema)
	if err != nil {
		return nil, err
	}

	inputSection := sectionHeader + "\n\n" + propertiesTable + "```json\n" + jsonSchema.String() + "\n```"
	r := regexp.MustCompile(fmt.Sprintf("%s\n+(.|\\s)*```", sectionHeader))
	// If the section already exists, replace it
	if r.Match(src) {
		return r.ReplaceAllLiteral(src, []byte(inputSection)), nil
	}

	// Append it
	return append(src, []byte("\n\n"+inputSection)...), nil
}

func renderSchemaTable(schemaRaw []byte) (string, error) {
	schema, err := sdk.CompileJSONSchema(schemaRaw)
	if err != nil {
		return "", fmt.Errorf("failed to compile schema: %w", err)
	}

	properties := make(sdk.SchemaPropertiesMap)
	err = sdk.CalculatePropertiesMap(schema, &properties)
	if err != nil {
		return "", fmt.Errorf("failed to calculate properties map: %w", err)
	}

	if len(properties) == 0 {
		return "", nil
	}

	table := "|Field|Type|Required|Description|\n|---|---|---|---|\n"

	// Sort map
	keys := make([]string, 0, len(properties))
	for k := range properties {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		v := properties[k]

		propertyType := v.Type
		if v.Format != "" {
			propertyType = fmt.Sprintf("%s (%s)", propertyType, v.Format)
		}

		required := "no"
		if v.Required {
			required = "yes"
		}

		table += fmt.Sprintf("|%s|%s|%s|%s|\n", v.Name, propertyType, required, v.Description)
	}

	return table + "\n", nil
}
