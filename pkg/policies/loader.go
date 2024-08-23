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

package policies

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/sigstore/cosign/v2/pkg/blob"
	"google.golang.org/protobuf/encoding/protojson"
)

// Loader defines the interface for policy loaders from contract attachments
type Loader interface {
	Load(context.Context, *v1.PolicyAttachment) (*v1.Policy, error)
}

// EmbeddedLoader returns embedded policies
type EmbeddedLoader struct{}

func (e *EmbeddedLoader) Load(_ context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, error) {
	return attachment.GetEmbedded(), nil
}

// BlobLoader loader loads policies from filesystem and HTTPS references using Cosign's blob package
type BlobLoader struct{}

func (l *BlobLoader) Load(_ context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, error) {
	var (
		rawData []byte
		err     error
	)

	reference := attachment.GetRef()

	// Support file:// references
	parts := strings.SplitAfterN(reference, "://", 2)
	if len(parts) == 2 && parts[0] == "file://" {
		rawData, err = os.ReadFile(filepath.Clean(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("loading policy spec: %w", err)
		}
	}

	// this method understands env, http and https schemes, and defaults to file system (without scheme).
	if rawData == nil {
		rawData, err = blob.LoadFileOrURL(reference)
		if err != nil {
			return nil, fmt.Errorf("loading policy spec: %w", err)
		}
	}

	jsonContent, err := materials.LoadJSONBytes(rawData, filepath.Ext(reference))
	if err != nil {
		return nil, fmt.Errorf("loading policy spec: %w", err)
	}

	var spec v1.Policy
	if err := protojson.Unmarshal(jsonContent, &spec); err != nil {
		return nil, fmt.Errorf("unmarshalling policy spec: %w", err)
	}
	return &spec, nil
}

const ChainloopScheme = "chainloop"

// ChainloopLoader loads policies referenced with chainloop://provider/name URLs
type ChainloopLoader struct {
	Client pb.AttestationServiceClient

	cacheMutex sync.Mutex
}

var remotePolicyCache = make(map[string]*v1.Policy)

func NewChainloopLoader(client pb.AttestationServiceClient) *ChainloopLoader {
	return &ChainloopLoader{Client: client}
}

func (c *ChainloopLoader) Load(ctx context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, error) {
	ref := attachment.GetRef()

	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	if remotePolicyCache[ref] != nil {
		return remotePolicyCache[ref], nil
	}

	if !IsProviderScheme(ref) {
		return nil, fmt.Errorf("invalid policy reference %q", ref)
	}

	provider, name := ProviderParts(ref)

	resp, err := c.Client.GetPolicy(ctx, &pb.AttestationServiceGetPolicyRequest{
		Provider:   provider,
		PolicyName: name,
	})
	if err != nil {
		return nil, fmt.Errorf("requesting remote policy (provider: %s, name: %s): %w", provider, name, err)
	}

	// cache result
	remotePolicyCache[ref] = resp.GetPolicy()

	return resp.GetPolicy(), nil
}

// IsProviderScheme takes a policy reference and returns whether it's referencing to an external provider or not
func IsProviderScheme(ref string) bool {
	parts := strings.SplitN(ref, "://", 2)
	return len(parts) == 2 && parts[0] == ChainloopScheme
}

func ProviderParts(ref string) (string, string) {
	parts := strings.SplitN(ref, "://", 2)
	pn := strings.SplitN(parts[1], "/", 2)
	var (
		name     = pn[0]
		provider string
	)
	if len(pn) == 2 {
		provider = pn[0]
		name = pn[1]
	}
	return provider, name
}
