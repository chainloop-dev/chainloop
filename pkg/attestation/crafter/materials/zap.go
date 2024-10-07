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
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"

	"github.com/rs/zerolog"
)

const (
	// zapReportFileName is the name of the file that contains the ZAP report in JSON format
	zapReportFileName = "report_json.json"
	// zapProgramName is the name of the program that generated the ZAP report
	zapProgramName = "ZAP"
)

// zapJSON is the structure of the ZAP report JSON with the values being checked
type zapJSON struct {
	ProgramName string `json:"@programName"`
}

type ZAPCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewZAPCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*ZAPCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_ZAP_DAST_ZIP {
		return nil, fmt.Errorf("material type is not zap dast zip file")
	}

	return &ZAPCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

// Craft will extract the ZAP JSON report from the zip file and upload it to the CAS
func (i *ZAPCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the zip file: %w", err)
	}
	defer archive.Close()

	var rawZapReport []byte

	// Iterate over files in the zip archive
	for _, f := range archive.File {
		if f.FileInfo().Name() != zapReportFileName || f.FileInfo().IsDir() {
			continue
		}

		// Open the file inside the zip
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("can't open the file: %w", err)
		}
		defer rc.Close()

		// Read the content of the file
		rawZapReport, err = io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("can't read the file: %w", err)
		}
		break
	}

	// Check if the ZAP report was found
	if len(rawZapReport) == 0 {
		return nil, fmt.Errorf("zip file does not contain the ZAP report: %w", ErrInvalidMaterialType)
	}

	// Parse the ZAP JSON report
	var zapJSONResult zapJSON
	if err = json.Unmarshal(rawZapReport, &zapJSONResult); err != nil {
		return nil, fmt.Errorf("invalid ZAP report file: %w", ErrInvalidMaterialType)
	}

	// Validate the report content
	if zapJSONResult.ProgramName != zapProgramName {
		return nil, fmt.Errorf("invalid ZAP report file: %w", ErrInvalidMaterialType)
	}

	// Call uploadAndCraft with the path of the JSON report file
	return uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
}
