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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	fileScheme      = "file"
	httpScheme      = "http"
	httpsScheme     = "https"
	chainloopScheme = "chainloop"
)

// Loader defines the interface for policy loaders from contract attachments
type Loader interface {
	Load(context.Context, *v1.PolicyAttachment) (*v1.Policy, string, error)
}

// EmbeddedLoader returns embedded policies
type EmbeddedLoader struct{}

const ChainloopLoaderID = "chainloop"

func (e *EmbeddedLoader) Load(_ context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, string, error) {
	return attachment.GetEmbedded(), attachment.GetRef(), nil
}

// FileLoader loader loads policies from filesystem and HTTPS references using Cosign's blob package
type FileLoader struct{}

func (l *FileLoader) Load(_ context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, string, error) {
	var (
		rawData []byte
		err     error
	)

	ref := attachment.GetRef()
	filePath, err := ensureScheme(ref, fileScheme)
	if err != nil {
		return nil, "", fmt.Errorf("invalid policy reference %q", ref)
	}

	rawData, err = os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, "", fmt.Errorf("loading policy spec: %w", err)
	}

	p, err := unmarshalPolicy(rawData, filepath.Ext(ref))
	if err != nil {
		return nil, "", fmt.Errorf("unmarshalling policy spec: %w", err)
	}

	return p, ref, nil
}

// HTTPSLoader loader loads policies from HTTP or HTTPS references
type HTTPSLoader struct{}

func (l *HTTPSLoader) Load(_ context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, string, error) {
	ref := attachment.GetRef()

	// and do not remove the scheme since we need http(s):// to make the request
	if _, err := ensureScheme(ref, httpScheme, httpsScheme); err != nil {
		return nil, "", fmt.Errorf("invalid policy reference %q: %w", ref, err)
	}

	// #nosec G107
	resp, err := http.Get(ref)
	if err != nil {
		return nil, "", fmt.Errorf("requesting remote policy: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading remote policy: %w", err)
	}

	p, err := unmarshalPolicy(raw, filepath.Ext(ref))
	if err != nil {
		return nil, "", fmt.Errorf("unmarshalling policy spec: %w", err)
	}

	return p, ref, nil
}

func unmarshalPolicy(rawData []byte, ext string) (*v1.Policy, error) {
	jsonContent, err := materials.LoadJSONBytes(rawData, ext)
	if err != nil {
		return nil, fmt.Errorf("loading policy spec: %w", err)
	}

	var spec v1.Policy
	if err := protojson.Unmarshal(jsonContent, &spec); err != nil {
		return nil, fmt.Errorf("unmarshalling policy spec: %w", err)
	}

	return &spec, nil
}

// ChainloopLoader loads policies referenced with chainloop://provider/name URLs
type ChainloopLoader struct {
	Client pb.AttestationServiceClient

	cacheMutex sync.Mutex
}

var remotePolicyCache = make(map[string]*pb.AttestationServiceGetPolicyResponse)

func NewChainloopLoader(client pb.AttestationServiceClient) *ChainloopLoader {
	return &ChainloopLoader{Client: client}
}

func (c *ChainloopLoader) Load(ctx context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, string, error) {
	ref := attachment.GetRef()

	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	if v, ok := remotePolicyCache[ref]; ok {
		return v.GetPolicy(), v.GetReference(), nil
	}

	if !IsProviderScheme(ref) {
		return nil, "", fmt.Errorf("invalid policy reference %q", ref)
	}

	provider, name := ProviderParts(ref)

	resp, err := c.Client.GetPolicy(ctx, &pb.AttestationServiceGetPolicyRequest{
		Provider:   provider,
		PolicyName: name,
	})
	if err != nil {
		return nil, "", fmt.Errorf("requesting remote policy (provider: %s, name: %s): %w", provider, name, err)
	}

	// cache result
	remotePolicyCache[ref] = resp

	return resp.GetPolicy(), resp.GetReference(), nil
}

// IsProviderScheme takes a policy reference and returns whether it's referencing to an external provider or not
func IsProviderScheme(ref string) bool {
	scheme, _ := refParts(ref)
	return scheme == chainloopScheme || scheme == ""
}

func ProviderParts(ref string) (string, string) {
	parts := strings.SplitN(ref, "://", 2)
	var pn []string
	if len(parts) == 1 {
		pn = strings.SplitN(parts[0], "/", 2)
	} else {
		// ref might contain the chainloop://protocol
		pn = strings.SplitN(parts[1], "/", 2)
	}

	var (
		provider string
		name     = pn[0]
	)
	if len(pn) == 2 {
		provider = pn[0]
		name = pn[1]
	}
	return provider, name
}

func ensureScheme(ref string, expected ...string) (string, error) {
	scheme, id := refParts(ref)
	for _, ex := range expected {
		if scheme == ex {
			return id, nil
		}
	}

	return "", fmt.Errorf("unexpected policy reference scheme: %q", scheme)
}

func refParts(ref string) (string, string) {
	parts := strings.SplitN(ref, "://", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}
