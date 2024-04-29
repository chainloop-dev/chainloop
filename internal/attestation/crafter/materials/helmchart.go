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
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"
)

const (
	// chartFileName is the name of the Chart.yaml file in the helm chart
	chartFileName = "Chart.yaml"
	// chartValuesYamlFileName is the name of the values.yaml file in the helm chart
	chartValuesYamlFileName = "values.yaml"
)

type HelmChartCrafter struct {
	backend *casclient.CASBackend
	*crafterCommon
}

func NewHelmChartCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend,
	l *zerolog.Logger) (*HelmChartCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_HELM_CHART {
		return nil, fmt.Errorf("material type is not HELM_CHART format")
	}

	return &HelmChartCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

func (c *HelmChartCrafter) Craft(ctx context.Context, filepath string) (*api.Attestation_Material, error) {
	// Open the helm chart tar file
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	// Decompress the file if possible
	uncompressedStream, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("can't uncompress file, unexpected material type: %w", err)
	}

	// Create a tar reader
	tarReader := tar.NewReader(uncompressedStream)

	// Flags to track whether required files are found
	var chartFileValid, chartValuesValid bool

	// Iterate through the files in the tar archive
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			// Reached the end of tar archive
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading tar file: %w", err)
		}

		// Check if the file is a regular file
		if header.Typeflag != tar.TypeReg {
			continue // Skip if it's not a regular file
		}

		// Validate Chart.yaml and values.yaml files. The files will have prepended the path of the directory
		// it was compressed from. So, we can check if the file name contains the required file names
		// Ex: helm-chart/Chart.yaml, helm-chart/values.yaml
		if strings.Contains(header.Name, chartFileName) {
			if err := c.validateYamlFile(tarReader); err != nil {
				return nil, fmt.Errorf("invalid Chart.yaml file: %w", err)
			}
			chartFileValid = true
		} else if strings.Contains(header.Name, chartValuesYamlFileName) {
			if err := c.validateYamlFile(tarReader); err != nil {
				return nil, fmt.Errorf("invalid values.yaml file: %w", err)
			}
			chartValuesValid = true
		}

		// Stop iterating if both files are found
		if chartValuesValid && chartFileValid {
			break
		}
	}

	// If the chart.yaml and values.yaml files are not found, return an error
	if !chartFileValid || !chartValuesValid {
		return nil, fmt.Errorf("missing required files in the helm chart: Chart.yaml and values.yaml")
	}

	// Upload and craft the chart
	return uploadAndCraftFromFile(ctx, c.input, c.backend, filepath, c.logger)
}

// validateYamlFile validates the YAML file just by trying to unmarshal it
func (c *HelmChartCrafter) validateYamlFile(r io.Reader) error {
	v := make(map[string]interface{})
	if err := yaml.NewDecoder(r).Decode(v); err != nil {
		return fmt.Errorf("failed to unmarshal YAML file: %w", err)
	}

	return nil
}
