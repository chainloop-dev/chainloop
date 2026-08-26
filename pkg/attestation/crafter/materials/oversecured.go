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

package materials

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

type OversecuredCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// oversecuredReport is the subset of the Oversecured whole-scan JSON export
// (`oversecured report <scanId> --app <appId> --format json`, or
// `GET …/scans/{scanId}/export?format=json`) used to validate its structure.
// The vendor does not publish this envelope, so it is pinned from a real export.
// https://docs.oversecured.com/docs/guide-exporting-reports
//
// Vulnerabilities is kept as a json.RawMessage for two reasons. It tells an
// absent "vulnerabilities" key (look-alike JSON) apart from a present-but-null
// or empty one (a clean scan) — encoding/json decodes both an absent key and an
// explicit null into a nil typed slice, so a typed field alone cannot
// distinguish them. It also leaves the findings themselves unparsed: they carry
// decompiled sources and DAST heap snapshots and make up nearly all of the
// file, while identifying the tool no better than the header already does.
type oversecuredReport struct {
	Header          oversecuredHeader `json:"header"`
	Vulnerabilities json.RawMessage   `json:"vulnerabilities"`
}

type oversecuredHeader struct {
	Scan oversecuredScan `json:"scan"`
	App  oversecuredApp  `json:"app"`
}

type oversecuredScan struct {
	ID string `json:"id"`
}

type oversecuredApp struct {
	Platform string `json:"platform"`
}

// The platforms Oversecured scans today. An export naming something else is
// still accepted — see Craft — so a new vendor platform does not need a
// Chainloop release to unblock pipelines.
const (
	oversecuredPlatformAndroid = "android"
	oversecuredPlatformIOS     = "ios"
)

func NewOversecuredCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*OversecuredCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_OVERSECURED_JSON {
		return nil, fmt.Errorf("material type is not an Oversecured JSON report")
	}
	craftCommon := &crafterCommon{logger: l, input: schema}
	return &OversecuredCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

func (i *OversecuredCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	var report oversecuredReport
	if err = json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("invalid Oversecured JSON report: %w", ErrInvalidMaterialType)
	}

	// Structural fingerprint check, deliberately limited to the export envelope:
	// a scan id, an app platform, and a findings array. Unlike the findings, the
	// envelope costs the same to check no matter how large the report is.
	// Auto-detection is intentionally disabled for this kind, but we still reject
	// look-alike JSON so an explicit --kind OVERSECURED_JSON fails loudly on the
	// wrong file — including on the paginated findings list, which carries no
	// header and holds a single page of findings (20 by default).
	if report.Header.Scan.ID == "" {
		return nil, fmt.Errorf("missing scan id in Oversecured report: %w", ErrInvalidMaterialType)
	}

	// Only the presence of a platform is required, not its value: Oversecured
	// scans mobile binaries exclusively, so an export always names one, but
	// hard-coding the vocabulary would mean a new vendor platform breaks every
	// pipeline until Chainloop ships a release.
	platform := strings.ToLower(report.Header.App.Platform)
	if platform == "" {
		return nil, fmt.Errorf("missing app platform in Oversecured report: %w", ErrInvalidMaterialType)
	}

	if platform != oversecuredPlatformAndroid && platform != oversecuredPlatformIOS {
		i.logger.Debug().Str("platform", report.Header.App.Platform).Msg("unfamiliar Oversecured app platform")
	}

	if report.Vulnerabilities == nil {
		return nil, fmt.Errorf("missing vulnerabilities in Oversecured report: %w", ErrInvalidMaterialType)
	}

	// The findings must be an array, or null for a clean scan — rejecting a clean
	// scan would break pipelines whose app simply has no findings, the common
	// case in CI. The unmarshal above already validated the whole document, so
	// the first byte settles the shape without parsing the findings a second
	// time: they are decompiled sources and DAST heap snapshots, nearly all of a
	// multi-MB export. Valid JSON leaves only one value starting with 'n'.
	if v := bytes.TrimLeft(report.Vulnerabilities, " \t\r\n"); len(v) == 0 || (v[0] != '[' && v[0] != 'n') {
		return nil, fmt.Errorf("vulnerabilities is not an array in Oversecured report: %w", ErrInvalidMaterialType)
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m)

	return m, nil
}

func (i *OversecuredCrafter) injectAnnotations(m *api.Attestation_Material) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = "oversecured"

	// No tool version and no scan.types annotation: the export carries neither a
	// scanner version (header.scan.version is the *app* version) nor the
	// per-finding hasSast/hasDast flags that would say which engines produced the
	// findings. Those flags exist only on the findings-list and single-finding API
	// responses. Nothing in the export states which engines ran either, so a DAST
	// scan that found nothing is indistinguishable from one that never ran —
	// asserting a scan type here would be a guess on a compliance signal.
}
