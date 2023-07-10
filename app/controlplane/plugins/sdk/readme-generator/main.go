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

//go:generate go run main.go --dir ../../core --integrations-index-path ../../../../../docs/integrations.md

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
	"strings"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
)

// base path to the plugins directory
var pluginsDir string
var integrationsIndexPath string

// Enhance README.md files for the registrations with the registration and attachment input schemas
func mainE() error {
	l := log.NewStdLogger(os.Stdout)

	plugins, err := plugins.Load("", l)
	if err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	// Update the list of available plugins
	// Update each readme file
	for _, e := range plugins {
		if err := updatePluginReadme(e); err != nil {
			return fmt.Errorf("failed to update README.md file: %w", err)
		}
	}

	// Update integrations index file
	if err := updateIntegrationsIndex(plugins); err != nil {
		return fmt.Errorf("failed to update integrations index: %w", err)
	}

	return nil
}

func updateIntegrationsIndex(plugins sdk.AvailablePlugins) error {
	const indexHeader = "## Available Integrations"

	// Find README integrationsIndex and extract its content
	indexFile, err := os.OpenFile(integrationsIndexPath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open index file %q: %w", integrationsIndexPath, err)
	}
	defer indexFile.Close()

	fileContent, err := io.ReadAll(indexFile)
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", integrationsIndexPath, err)
	}

	indexTable := "| ID | Version | Description | Material Requirement |\n| --- | --- | --- | --- |\n"
	for _, p := range plugins {
		info := p.Describe()
		// Load the materials
		var subscribedMaterials = make([]string, 0)
		for _, m := range info.SubscribedMaterials {
			subscribedMaterials = append(subscribedMaterials, m.Type.String())
		}

		// We need to full URL path because we render this file in the website
		const repoBase = "https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/core"
		pathToPlugin := fmt.Sprintf("%s/%s/%s/%s", repoBase, p.Describe().ID, "v1", "README.md")

		indexTable += fmt.Sprintf("| [%s](%s) | %s | %s | %s |\n", info.ID, pathToPlugin, info.Version, info.Description, strings.Join(subscribedMaterials, ", "))
	}

	// Replace the table
	section := indexHeader + "\n\n" + indexTable + "\n"
	// Find the content that starts with the indexHeader and contains a markdown table
	// letters, |, _, -, \n, separators (i.e comma, {, [, ...]}), ... are allowed between the indexHeader and the table
	r := regexp.MustCompile(fmt.Sprintf("%s\n*[\\w|\\||\\-|\\s|\\.|_|\\,|\\[|\\]|\\(|\\)|\\/|:]*", indexHeader))

	fileContent = r.ReplaceAllLiteral(fileContent, []byte(section))

	return truncateAndWriteFile(indexFile, fileContent)
}

func truncateAndWriteFile(f *os.File, content []byte) error {
	// Write the new content in the file
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}

	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	_, err := f.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write file %q: %w", integrationsIndexPath, err)
	}

	return nil
}

func updatePluginReadme(p sdk.FanOut) error {
	const registrationInputHeader = "## Registration Input Schema"
	const attachmentInputHeader = "## Attachment Input Schema"

	// Find README file and extract its content
	file, err := os.OpenFile(filepath.Join(pluginsDir, p.Describe().ID, "v1", "README.md"), os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open README.md file: %w", err)
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read README.md file: %w", err)
	}

	// Replace/Add registration input schema
	fileContent, err = addSchemaToSection(fileContent, registrationInputHeader, p.Describe().RegistrationJSONSchema)
	if err != nil {
		return fmt.Errorf("failed to add registration schema to README.md file: %w", err)
	}

	// Replace/Add attachment input schema
	fileContent, err = addSchemaToSection(fileContent, attachmentInputHeader, p.Describe().AttachmentJSONSchema)
	if err != nil {
		return fmt.Errorf("failed to add attachment schema to README.md file: %w", err)
	}

	return truncateAndWriteFile(file, fileContent)
}

func main() {
	if err := mainE(); err != nil {
		panic(err)
	}
}

func init() {
	flag.StringVar(&pluginsDir, "dir", "", "base directory for plugins i.e ./core")
	flag.StringVar(&integrationsIndexPath, "integrations-index-path", "", "integrations list markdown file i.e docs/integrations.md")
	flag.Parse()
}

func addSchemaToSection(src []byte, sectionHeader string, schema []byte) ([]byte, error) {
	var jsonSchema bytes.Buffer
	err := json.Indent(&jsonSchema, schema, "", "  ")
	if err != nil {
		return nil, err
	}

	s, err := sdk.CompileJSONSchema(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	properties := make(sdk.SchemaPropertiesMap)
	err = sdk.CalculatePropertiesMap(s, &properties)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate properties map: %w", err)
	}

	// Return original content if there is no properties
	if len(properties) == 0 {
		return src, nil
	}

	propertiesTable, err := renderSchemaTable(properties)
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

func renderSchemaTable(properties sdk.SchemaPropertiesMap) (string, error) {
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
