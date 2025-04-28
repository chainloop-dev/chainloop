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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/cmd"
	"github.com/rs/zerolog"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

const fileHeader = `---
title: CLI
---

Chainloop CLI is a command-line tool designed to streamline the process of crafting, managing, and storing software supply chain attestations. The CLI enables developers to generate and submit evidence-such as build artifacts, SBOMs, 
and vulnerability reports-directly from their CI/CD workflows, ensuring compliance with organizational policies without introducing friction into the development process.

The CLI operates through a contract-based workflow. Security teams define workflow contracts specifying which types of evidence must be collected and how they should be validated. Developers then use the Chainloop CLI to initialize 
an attestation, add the required materials, and submit the attestation for validation and storage. Each command can accept arguments as traditional flags or as environment variables

`

//go:generate go run ./generate.go ./
func main() {
	if len(os.Args) != 2 {
		log.Fatal("Required argument: cli docs output directory")
	}
	out := os.Args[1]

	command := cmd.NewRootCmd(zerolog.Nop())
	command.Use = "chainloop [command]"
	command.DisableAutoGenTag = true

	var builder strings.Builder
	for _, subCmd := range command.Commands() {
		if !subCmd.Hidden {
			// Start depth at 0 for subcommands
			generateCommandDocs(subCmd, &builder, 0)
		}
	}

	formatted := processFinalDocument(builder.String())
	withHeader := fmt.Sprintf("%s%s", fileHeader, formatted)

	err := os.WriteFile(filepath.Join(out, "cli-reference.mdx"), []byte(withHeader), 0755)
	if err != nil {
		log.Fatal(err)
	}
}

// generateCommandDocs recursively generates documentation for a command and its subcommands.
func generateCommandDocs(cmd *cobra.Command, builder *strings.Builder, currentDepth int) {
	var cmdBuffer strings.Builder
	// Generate base documentation
	if err := doc.GenMarkdown(cmd, &cmdBuffer); err != nil {
		log.Fatal(fmt.Errorf("failed to generate documentation: %w", err))
	}
	processed := processCommand(cmdBuffer.String(), currentDepth)
	builder.WriteString(processed)

	// Recursively process subcommands with increased depth
	for _, subCmd := range cmd.Commands() {
		if !subCmd.Hidden {
			generateCommandDocs(subCmd, builder, currentDepth+1)
		}
	}
}

// processCommand processes the command documentation, filtering out unnecessary sections and formatting headers.
func processCommand(content string, depth int) string {
	lines := strings.Split(content, "\n")
	var filtered []string
	inSeeAlso := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip "SEE ALSO" section and everything after
		if strings.Contains(trimmed, "### SEE ALSO") {
			inSeeAlso = true
			continue
		}
		if inSeeAlso {
			continue
		}

		// Strip all leading '#' characters and spaces from any header line
		cleanLine := strings.TrimLeft(trimmed, "# ")

		// Detect underline-style headers (--- or ===)
		if i > 0 && isUnderlineHeader(line) {
			// Remove previous line (header text)
			if len(filtered) > 0 {
				prev := filtered[len(filtered)-1]
				filtered = filtered[:len(filtered)-1]
				// Add heading with dynamic depth + 1 (subsection)
				filtered = append(filtered, fmt.Sprintf("%s %s",
					strings.Repeat("#", depth+3), // depth+3 for sub-sections
					prev))
			}
			continue
		}

		// For the first line (command title), apply heading with dynamic depth
		if i == 0 && cleanLine != "" {
			filtered = append(filtered, fmt.Sprintf("%s %s",
				strings.Repeat("#", depth+2), // depth+2 to start from H2 for root
				cleanLine))
			continue
		}

		// Replace $HOME environment variable references
		if home := os.Getenv("HOME"); home != "" {
			cleanLine = strings.ReplaceAll(cleanLine, home, "$HOME")
		}

		filtered = append(filtered, cleanLine)
	}

	return strings.Join(filtered, "\n") + "\n\n"
}

// isUnderlineHeader checks if a line is an underline-style header (--- or ===).
func isUnderlineHeader(line string) bool {
	return regexp.MustCompile(`^[-=]+$`).MatchString(line)
}

// processFinalDocument cleans up the final document by removing excessive newlines and headers.
func processFinalDocument(content string) string {
	// Clean up excessive newlines and leftover headers
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")
	return regexp.MustCompile(`(?m)^#+\s*$`).ReplaceAllString(content, "")
}
