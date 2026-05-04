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
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

type GraphQLCrafter struct {
	backend *casclient.CASBackend
	*crafterCommon
}

func NewGraphQLCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*GraphQLCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_GRAPHQL_SPEC {
		return nil, fmt.Errorf("material type is not GRAPHQL_SPEC format")
	}

	return &GraphQLCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

func (i *GraphQLCrafter) Craft(ctx context.Context, filepath string) (*api.Attestation_Material, error) {
	i.logger.Debug().Str("path", filepath).Msg("decoding GraphQL SDL file")

	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	source := &ast.Source{Name: filepath, Input: string(content)}
	doc, parseErr := parser.ParseSchema(source)
	if parseErr != nil {
		i.logger.Debug().Err(parseErr).Msg("error decoding file")
		return nil, fmt.Errorf("invalid GraphQL SDL file: %w", ErrInvalidMaterialType)
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filepath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, doc)

	return m, nil
}

func (i *GraphQLCrafter) injectAnnotations(m *api.Attestation_Material, doc *ast.SchemaDocument) {
	m.Annotations = make(map[string]string)

	m.Annotations["chainloop.material.graphql.type_count"] = strconv.Itoa(len(doc.Definitions))

	if len(doc.Directives) > 0 {
		names := make([]string, 0, len(doc.Directives))
		for _, d := range doc.Directives {
			names = append(names, d.Name)
		}
		sort.Strings(names)
		m.Annotations["chainloop.material.graphql.directives"] = strings.Join(names, ",")
	}
}
