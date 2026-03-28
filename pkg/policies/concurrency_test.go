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

package policies

import (
	"context"
	"os"
	"sync"
	"testing"

	v12 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestConcurrentVerifyStatement runs VerifyStatement from multiple goroutines
// to exercise errgroup parallelism and catch race conditions.
// Run with: go test -race -count=1 -run TestConcurrentVerifyStatement ./pkg/policies/...
func TestConcurrentVerifyStatement(t *testing.T) {
	logger := zerolog.Nop()

	schema := &v12.CraftingSchema{
		Policies: &v12.Policies{
			Attestation: []*v12.PolicyAttachment{
				{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/workflow.yaml"}},
				{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/materials.yaml"}},
			},
		},
	}

	statement := loadStatementForTest(t, "testdata/statement.json")

	pv := NewPolicyVerifier(schema.GetPolicies(), nil, &logger)

	// Run multiple VerifyStatement calls concurrently
	const goroutines = 10
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	results := make([][]*v1.PolicyEvaluation, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := pv.VerifyStatement(context.Background(), statement)
			errs[i] = err
			results[i] = res
		}()
	}

	wg.Wait()

	// All calls should succeed
	for i := range goroutines {
		assert.NoError(t, errs[i], "goroutine %d failed", i)
	}

	// All calls should return the same number of evaluations
	for i := 1; i < goroutines; i++ {
		assert.Equal(t, len(results[0]), len(results[i]),
			"goroutine %d returned different number of evaluations", i)
	}
}

// TestConcurrentVerifyMaterial runs VerifyMaterial from multiple goroutines.
func TestConcurrentVerifyMaterial(t *testing.T) {
	logger := zerolog.Nop()

	schema := &v12.CraftingSchema{
		Policies: &v12.Policies{
			Materials: []*v12.PolicyAttachment{
				{
					Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft.yaml"},
				},
			},
		},
	}

	material := &v1.Attestation_Material{
		Id:           "sbom",
		MaterialType: v12.CraftingSchema_Material_SBOM_SPDX_JSON,
		M: &v1.Attestation_Material_Artifact_{
			Artifact: &v1.Attestation_Material_Artifact{},
		},
	}

	pv := NewPolicyVerifier(schema.GetPolicies(), nil, &logger)

	const goroutines = 10
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	results := make([][]*v1.PolicyEvaluation, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := pv.VerifyMaterial(context.Background(), material, "testdata/sbom-spdx.json")
			errs[i] = err
			results[i] = res
		}()
	}

	wg.Wait()

	for i := range goroutines {
		assert.NoError(t, errs[i], "goroutine %d failed", i)
	}

	for i := 1; i < goroutines; i++ {
		assert.Equal(t, len(results[0]), len(results[i]),
			"goroutine %d returned different number of evaluations", i)
	}
}

// TestWithMaxConcurrency verifies the WithMaxConcurrency option is applied.
func TestWithMaxConcurrency(t *testing.T) {
	logger := zerolog.Nop()

	// Test default
	pv := NewPolicyVerifier(nil, nil, &logger)
	assert.Greater(t, pv.maxConcurrency, 0, "default maxConcurrency should be positive")

	// Test custom value
	pv = NewPolicyVerifier(nil, nil, &logger, WithMaxConcurrency(5))
	assert.Equal(t, 5, pv.maxConcurrency)

	// Test zero defaults to computed default
	pv = NewPolicyVerifier(nil, nil, &logger, WithMaxConcurrency(0))
	assert.Equal(t, defaultMaxConcurrency, pv.maxConcurrency)

	// Test negative defaults to computed default
	pv = NewPolicyVerifier(nil, nil, &logger, WithMaxConcurrency(-1))
	assert.Equal(t, defaultMaxConcurrency, pv.maxConcurrency)
}

// TestVerifyStatementWithConcurrencyOne ensures sequential mode (maxConcurrency=1)
// produces identical results to default parallel mode.
func TestVerifyStatementWithConcurrencyOne(t *testing.T) {
	logger := zerolog.Nop()

	schema := &v12.CraftingSchema{
		Policies: &v12.Policies{
			Attestation: []*v12.PolicyAttachment{
				{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/workflow.yaml"}},
				{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/materials.yaml"}},
			},
		},
	}

	statement := loadStatementForTest(t, "testdata/statement.json")

	// Sequential
	pvSeq := NewPolicyVerifier(schema.GetPolicies(), nil, &logger, WithMaxConcurrency(1))
	seqResults, seqErr := pvSeq.VerifyStatement(context.Background(), statement)

	// Parallel (default)
	pvPar := NewPolicyVerifier(schema.GetPolicies(), nil, &logger)
	parResults, parErr := pvPar.VerifyStatement(context.Background(), statement)

	require.NoError(t, seqErr)
	require.NoError(t, parErr)
	assert.Equal(t, len(seqResults), len(parResults),
		"sequential and parallel should return same number of evaluations")

	// Build name→result maps for comparison (order may differ)
	seqByName := make(map[string]*v1.PolicyEvaluation)
	for _, ev := range seqResults {
		seqByName[ev.Name] = ev
	}

	for _, ev := range parResults {
		seqEv, ok := seqByName[ev.Name]
		assert.True(t, ok, "parallel result has policy %q not found in sequential", ev.Name)
		if ok {
			assert.Equal(t, len(seqEv.Violations), len(ev.Violations),
				"policy %q: violation count mismatch", ev.Name)
			assert.Equal(t, seqEv.Skipped, ev.Skipped,
				"policy %q: skipped mismatch", ev.Name)
		}
	}
}

// TestErrGroupCancellation verifies that when one policy evaluation fails,
// the errgroup context is cancelled and propagated.
func TestErrGroupCancellation(t *testing.T) {
	logger := zerolog.Nop()

	schema := &v12.CraftingSchema{
		Policies: &v12.Policies{
			Attestation: []*v12.PolicyAttachment{
				{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/workflow.yaml"}},
				// This policy ref does not exist — will cause an error
				{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/nonexistent_policy.yaml"}},
			},
		},
	}

	statement := loadStatementForTest(t, "testdata/statement.json")

	pv := NewPolicyVerifier(schema.GetPolicies(), nil, &logger)
	_, err := pv.VerifyStatement(context.Background(), statement)

	assert.Error(t, err, "should fail when a policy file does not exist")
	assert.IsType(t, &PolicyError{}, err, "error should be a PolicyError")
}

func loadStatementForTest(t *testing.T, file string) *intoto.Statement {
	t.Helper()
	stContent, err := os.ReadFile(file)
	require.NoError(t, err)
	var statement intoto.Statement
	err = protojson.Unmarshal(stContent, &statement)
	require.NoError(t, err)
	return &statement
}
