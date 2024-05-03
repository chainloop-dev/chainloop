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

package materials

import (
	"context"
	"fmt"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rs/zerolog"
)

type OCIImageCrafter struct {
	*crafterCommon
	keychain authn.Keychain
}

func NewOCIImageCrafter(schema *schemaapi.CraftingSchema_Material, ociAuth authn.Keychain, l *zerolog.Logger) (*OCIImageCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_CONTAINER_IMAGE {
		return nil, fmt.Errorf("material type is not container image")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &OCIImageCrafter{craftCommon, ociAuth}, nil
}

func (i *OCIImageCrafter) Craft(_ context.Context, imageRef string) (*api.Attestation_Material, error) {
	i.logger.Debug().Str("name", imageRef).Msg("retrieving container image digest from remote")

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, err
	}

	descriptor, err := remote.Get(ref, remote.WithAuthFromKeychain(i.keychain))
	if err != nil {
		return nil, err
	}

	remoteRef := ref.Context().Digest(descriptor.Digest.String())

	// FQDN of the repo, i.e bitnami/nginx => index.docker.io/bitnami/nginx
	repoName := remoteRef.Repository.String()
	i.logger.Debug().Str("name", repoName).Str("digest", remoteRef.DigestStr()).Msg("container image resolved")

	return &api.Attestation_Material{
		MaterialType: i.input.Type,
		M: &api.Attestation_Material_ContainerImage_{
			ContainerImage: &api.Attestation_Material_ContainerImage{
				Id: i.input.Name, Name: repoName, Digest: remoteRef.DigestStr(), IsSubject: i.input.Output,
				Tag: ref.Identifier(),
			},
		},
	}, nil
}
