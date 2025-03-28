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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	fileScheme      = "file"
	httpScheme      = "http"
	httpsScheme     = "https"
	chainloopScheme = "chainloop"
)

// Loader defines the interface for policy loaders from contract attachments
type Loader interface {
	Load(context.Context, *v1.PolicyAttachment) (*v1.Policy, *PolicyDescriptor, error)
}

// PolicyDescriptor Represents a policy reference. Used as FQ references.
type PolicyDescriptor struct {
	// FQ URI of the policy
	URI string
	// Policy name (only when it can be resolved by the loader)
	Name string
	// policy digest
	Digest string
	// Org name for custom policies (only supported by the remote ChainloopLoader)
	OrgName string
}

func (p *PolicyDescriptor) GetURI() string {
	if p != nil {
		return p.URI
	}
	return ""
}

func (p *PolicyDescriptor) GetName() string {
	if p != nil {
		return p.Name
	}
	return ""
}

func (p *PolicyDescriptor) GetDigest() string {
	if p != nil {
		return p.Digest
	}
	return ""
}

func (p *PolicyDescriptor) GetOrgName() string {
	if p != nil {
		return p.OrgName
	}
	return ""
}

// EmbeddedLoader returns embedded policies
type EmbeddedLoader struct{}

func (e *EmbeddedLoader) Load(_ context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, *PolicyDescriptor, error) {
	return attachment.GetEmbedded(), nil, nil
}

// FileLoader loader loads policies from filesystem and HTTPS references using Cosign's blob package
type FileLoader struct{}

func (l *FileLoader) Load(_ context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, *PolicyDescriptor, error) {
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

	var policy v1.Policy
	d, err := unmarshallResource(raw, ref, wantDigest, &policy)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshalling policy spec: %w", err)
	}

	return &policy, d, nil
}

// HTTPSLoader loader loads policies from HTTP or HTTPS references
type HTTPSLoader struct{}

func (l *HTTPSLoader) Load(_ context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, *PolicyDescriptor, error) {
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

	var policy v1.Policy
	d, err := unmarshallResource(raw, ref, wantDigest, &policy)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshalling policy spec: %w", err)
	}

	return &policy, d, nil
}

// ChainloopLoader loads policies referenced with chainloop://provider/name URLs
type ChainloopLoader struct {
	Client pb.AttestationServiceClient

	cacheMutex sync.Mutex
}

type policyWithReference struct {
	policy    *v1.Policy
	reference *PolicyDescriptor
}

var remotePolicyCache = make(map[string]*policyWithReference)

func NewChainloopLoader(client pb.AttestationServiceClient) *ChainloopLoader {
	return &ChainloopLoader{Client: client}
}

func (c *ChainloopLoader) Load(ctx context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, *PolicyDescriptor, error) {
	ref := attachment.GetRef()

	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	if v, ok := remotePolicyCache[ref]; ok {
		return v.policy, v.reference, nil
	}

	if !IsProviderScheme(ref) {
		return nil, nil, fmt.Errorf("invalid policy reference %q", ref)
	}

	providerRef := ProviderParts(ref)

	resp, err := c.Client.GetPolicy(ctx, &pb.AttestationServiceGetPolicyRequest{
		Provider:   providerRef.Provider,
		PolicyName: providerRef.Name,
		OrgName:    providerRef.OrgName,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("requesting remote policy (provider: %s, name: %s): %w", providerRef.Provider, providerRef.Name, err)
	}

	h, err := crv1.NewHash(resp.Reference.GetDigest())
	if err != nil {
		return nil, nil, fmt.Errorf("parsing digest: %w", err)
	}

	orgName := providerRef.OrgName
	// Extract organization name from URL if present
	if u, err := url.Parse(resp.Reference.GetUrl()); err == nil {
		if orgParam := u.Query().Get("org"); orgParam != "" {
			orgName = orgParam
		}
	}

	reference := policyReferenceResourceDescriptor(providerRef.Name, resp.Reference.GetUrl(), orgName, h)

	// cache result
	remotePolicyCache[ref] = &policyWithReference{policy: resp.GetPolicy(), reference: reference}
	return resp.GetPolicy(), reference, nil
}

func unmarshallResource(raw []byte, ref string, digest string, dest proto.Message) (*PolicyDescriptor, error) {
	jsonContent, err := unmarshal.LoadJSONBytes(raw, filepath.Ext(ref))
	if err != nil {
		return nil, fmt.Errorf("loading resource spec: %w", err)
	}

	if err := protojson.Unmarshal(jsonContent, dest); err != nil {
		return nil, fmt.Errorf("unmarshalling policy spec: %w", err)
	}

	// calculate hash of the raw data
	h, _, err := crv1.SHA256(bytes.NewBuffer(raw))
	if err != nil {
		return nil, fmt.Errorf("calculating hash: %w", err)
	}

	// compare it with the wanted digest if needed
	if digest != "" && h.String() != digest {
		return nil, fmt.Errorf("digest mismatch: got %s, want %s", h.String(), digest)
	}

	return policyReferenceResourceDescriptor("", ref, "", h), nil
}

// IsProviderScheme takes a policy reference and returns whether it's referencing to an external provider or not
func IsProviderScheme(ref string) bool {
	scheme, _ := refParts(ref)
	return scheme == chainloopScheme || scheme == ""
}

// ProviderRef represents a policy provider reference
type ProviderRef struct {
	Provider, OrgName, Name string
}

// ProviderParts returns the provider information for a given reference
func ProviderParts(reference string) *ProviderRef {
	var ref, digest string
	// first of all, remove the @sha256 suffix to make the parsing easier
	withDigest := strings.SplitN(reference, "@sha256:", 2)

	if len(withDigest) > 1 {
		// it has digest
		ref = withDigest[0]
		digest = withDigest[1]
	} else {
		ref = reference
	}

	parts := strings.SplitN(ref, "://", 2)
	var pn []string
	if len(parts) == 1 {
		pn = strings.SplitN(parts[0], ":", 2)
	} else {
		// ref might contain the chainloop://protocol
		pn = strings.SplitN(parts[1], ":", 2)
	}

	var (
		provider string
		orgName  string
		name     = pn[0]
	)

	if len(pn) == 2 {
		provider = pn[0]
		name = pn[1]
	}
	scoped := strings.SplitN(name, "/", 2)
	if len(scoped) == 2 {
		// the policy is scoped to a specific org
		orgName = scoped[0]
		name = scoped[1]
	}

	// return the digest back
	if digest != "" {
		name = fmt.Sprintf("%s@sha256:%s", name, digest)
	}

	return &ProviderRef{
		Provider: provider,
		Name:     name,
		OrgName:  orgName,
	}
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

func policyReferenceResourceDescriptor(policyName, ref, orgName string, digest crv1.Hash) *PolicyDescriptor {
	return &PolicyDescriptor{
		URI:     ref,
		Name:    policyName,
		Digest:  digest.String(),
		OrgName: orgName,
	}
}

func ExtractDigest(ref string) (string, string) {
	parts := strings.SplitN(ref, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return parts[0], ""
}
