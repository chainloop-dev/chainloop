//
// Copyright 2024-2026 The Chainloop Authors.
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

package policies

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/cache"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"golang.org/x/sync/singleflight"
)

// GroupLoader defines the interface for policy loaders from contract attachments
type GroupLoader interface {
	Load(context.Context, *v1.PolicyGroupAttachment) (*v1.PolicyGroup, *PolicyDescriptor, error)
}

// FileGroupLoader loader loads policies from filesystem and HTTPS references using Cosign's blob package
type FileGroupLoader struct{}

func (l *FileGroupLoader) Load(_ context.Context, attachment *v1.PolicyGroupAttachment) (*v1.PolicyGroup, *PolicyDescriptor, error) {
	var (
		raw []byte
		err error
	)

	// First remove the digest if present
	ref, wantDigest := ExtractDigest(attachment.GetRef())
	filePath, err := ensureScheme(ref, fileScheme)
	if err != nil {
		return nil, nil, err
	}

	raw, err = os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, nil, fmt.Errorf("loading policy spec: %w", err)
	}

	var group v1.PolicyGroup
	d, err := unmarshallResource(raw, ref, wantDigest, &group)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshalling policy spec: %w", err)
	}

	return &group, d, nil
}

// HTTPSGroupLoader loader loads policies from HTTP or HTTPS references
type HTTPSGroupLoader struct{}

func (l *HTTPSGroupLoader) Load(_ context.Context, attachment *v1.PolicyGroupAttachment) (*v1.PolicyGroup, *PolicyDescriptor, error) {
	ref, wantDigest := ExtractDigest(attachment.GetRef())

	// and do not remove the scheme since we need http(s):// to make the request
	if _, err := ensureScheme(ref, httpScheme, httpsScheme); err != nil {
		return nil, nil, fmt.Errorf("invalid policy reference %q: %w", ref, err)
	}

	// #nosec G107
	resp, err := http.Get(ref)
	if err != nil {
		return nil, nil, fmt.Errorf("requesting remote policy: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading remote policy: %w", err)
	}

	var group v1.PolicyGroup
	d, err := unmarshallResource(raw, ref, wantDigest, &group)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshalling policy spec: %w", err)
	}

	return &group, d, nil
}

// ChainloopGroupLoader loads groups referenced with chainloop://provider/name URLs
type ChainloopGroupLoader struct {
	Client pb.AttestationServiceClient
	cache  cache.Cache[*groupWithReference]
	flight singleflight.Group
}

type groupWithReference struct {
	Group     *v1.PolicyGroup   `json:"group"`
	Reference *PolicyDescriptor `json:"reference"`
}

func NewChainloopGroupLoader(client pb.AttestationServiceClient, c cache.Cache[*groupWithReference]) *ChainloopGroupLoader {
	return &ChainloopGroupLoader{Client: client, cache: c}
}

func (c *ChainloopGroupLoader) Load(ctx context.Context, attachment *v1.PolicyGroupAttachment) (*v1.PolicyGroup, *PolicyDescriptor, error) {
	ref := attachment.GetRef()

	// Fast path: check cache
	if cached, ok, _ := c.cache.Get(ctx, ref); ok {
		return cached.Group, cached.Reference, nil
	}

	// Use singleflight to coalesce concurrent fetches for the same ref.
	// All operations inside use a fixed-timeout context independent of any
	// caller's deadline so that all singleflight waiters get uniform behavior.
	result, err, _ := c.flight.Do(ref, func() (interface{}, error) {
		sfCtx, cancel := context.WithTimeout(context.Background(), remoteLoaderFetchTimeout)
		defer cancel()

		// Double-check cache inside singleflight
		if cached, ok, _ := c.cache.Get(sfCtx, ref); ok {
			return cached, nil
		}

		if !IsProviderScheme(ref) {
			return nil, fmt.Errorf("invalid group reference %q", ref)
		}

		providerRef := ProviderParts(ref)

		resp, err := c.Client.GetPolicyGroup(sfCtx, &pb.AttestationServiceGetPolicyGroupRequest{
			Provider:  providerRef.Provider,
			GroupName: providerRef.Name,
			OrgName:   providerRef.OrgName,
		})
		if err != nil {
			return nil, fmt.Errorf("requesting remote group (provider: %s, name: %s): %w", providerRef.Provider, providerRef.Name, err)
		}

		h, err := crv1.NewHash(resp.Reference.GetDigest())
		if err != nil {
			return nil, fmt.Errorf("parsing digest: %w", err)
		}

		orgName := providerRef.OrgName
		if u, err := url.Parse(resp.Reference.GetUrl()); err == nil {
			if orgParam := u.Query().Get("org"); orgParam != "" {
				orgName = orgParam
			}
		}

		reference := policyReferenceResourceDescriptor(providerRef.Name, resp.Reference.GetUrl(), orgName, h)
		cached := &groupWithReference{Group: resp.GetGroup(), Reference: reference}

		_ = c.cache.Set(sfCtx, ref, cached)

		return cached, nil
	})
	if err != nil {
		return nil, nil, err
	}

	cached := result.(*groupWithReference)
	return cached.Group, cached.Reference, nil
}
