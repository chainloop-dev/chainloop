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

package action

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

const (
	KindContract = "Contract"
)

// ApplyResult holds the outcome of applying a single resource document
type ApplyResult struct {
	Kind  string
	Name  string
	Error error
}

// YAMLDoc holds a parsed YAML document with its kind and raw bytes
type YAMLDoc struct {
	Kind    string
	Name    string
	RawData []byte
}

// Apply handles applying resources from YAML files
type Apply struct {
	cfg *ActionsOpts
}

// NewApply creates a new Apply action
func NewApply(cfg *ActionsOpts) *Apply {
	return &Apply{cfg: cfg}
}

// Run applies all resources found in the given path (file or directory)
func (a *Apply) Run(ctx context.Context, path string) ([]*ApplyResult, error) {
	docs, err := ParseYAMLPath(path)
	if err != nil {
		return nil, err
	}

	// Apply contracts
	var results []*ApplyResult
	for _, doc := range docs {
		result := &ApplyResult{Kind: doc.Kind, Name: doc.Name}
		switch doc.Kind {
		case KindContract:
			if err := ApplyContractFromRawData(ctx, a.cfg.CPConnection, doc.Name, doc.RawData); err != nil {
				result.Error = err
			}
		default:
			result.Error = fmt.Errorf("unsupported kind %q", doc.Kind)
		}
		results = append(results, result)
	}

	return results, nil
}

// ParseYAMLPath collects all YAML files from a path (file or directory),
// reads them, and splits multi-document files into individual YAMLDoc entries.
func ParseYAMLPath(path string) ([]*YAMLDoc, error) {
	files, err := CollectYAMLFiles(path)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no YAML files found in %q", path)
	}

	var allDocs []*YAMLDoc
	for _, f := range files {
		rawData, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading file %s: %w", f, err)
		}

		docs, err := SplitYAMLDocuments(rawData)
		if err != nil {
			return nil, fmt.Errorf("parsing file %s: %w", f, err)
		}

		allDocs = append(allDocs, docs...)
	}

	return allDocs, nil
}

// ApplyContractFromRawData applies a single contract document using the gRPC client.
// It uses describe to check existence, then creates or updates accordingly.
func ApplyContractFromRawData(ctx context.Context, conn *grpc.ClientConn, name string, rawData []byte) error {
	client := pb.NewWorkflowContractServiceClient(conn)

	// Try to describe the contract to determine if we should create or update
	_, err := client.Describe(ctx, &pb.WorkflowContractServiceDescribeRequest{
		Name: name,
	})
	if err == nil {
		// Contract exists, perform update
		_, err := client.Update(ctx, &pb.WorkflowContractServiceUpdateRequest{
			Name:        name,
			RawContract: rawData,
		})
		if err != nil {
			return fmt.Errorf("failed to update contract %q: %w", name, err)
		}
		return nil
	}

	// Contract doesn't exist, perform create
	_, err = client.Create(ctx, &pb.WorkflowContractServiceCreateRequest{
		Name:        name,
		RawContract: rawData,
	})
	if err != nil {
		return fmt.Errorf("failed to create contract %q: %w", name, err)
	}

	return nil
}

// CollectYAMLFiles returns YAML file paths from the given path.
// If path is a file, it returns that file. If a directory, it walks recursively.
func CollectYAMLFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("accessing path %q: %w", path, err)
	}

	if !info.IsDir() {
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory %q: %w", path, err)
	}

	return files, nil
}

// SplitYAMLDocuments splits a potentially multi-document YAML file into individual documents,
// extracting kind and name from each.
func SplitYAMLDocuments(rawData []byte) ([]*YAMLDoc, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(rawData))

	var docs []*YAMLDoc
	for {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decoding YAML document: %w", err)
		}

		// Marshal node back to bytes for the per-resource apply
		docBytes, err := yaml.Marshal(&node)
		if err != nil {
			return nil, fmt.Errorf("marshalling YAML node: %w", err)
		}

		// Extract kind and name via partial unmarshal
		var header struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
		}
		if err := yaml.Unmarshal(docBytes, &header); err != nil {
			return nil, fmt.Errorf("extracting document kind: %w", err)
		}

		if header.Kind == "" {
			return nil, fmt.Errorf("missing 'kind' field in YAML document")
		}

		docs = append(docs, &YAMLDoc{
			Kind:    header.Kind,
			Name:    header.Metadata.Name,
			RawData: docBytes,
		})
	}

	return docs, nil
}
