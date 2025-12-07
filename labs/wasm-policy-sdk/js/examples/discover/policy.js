//
// Copyright 2025 The Chainloop Authors.
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

/**
 * Discover policy example that checks for policy violations in related attestations.
 *
 * This policy validates that container images do not have any related attestations
 * with policy violations. It uses the discover builtin from the Chainloop JS SDK
 * to explore the artifact graph and check attestation metadata.
 *
 * This example demonstrates:
 * - Extracting digest from container image metadata
 * - Using the SDK's discover() function to explore artifact relationships
 * - Checking for policy violations in related attestations
 * - Processing attestation metadata (name, project, organization)
 *
 * Build:
 *   npm install
 *   npm run build
 *
 * Test with container image:
 *   chainloop policy develop eval \
 *     --policy policy.yaml \
 *     --material docker://nginx:latest \
 *     --kind CONTAINER_IMAGE
 */

// Import directly from specific modules
const { getMaterialJSON } = require('../../src/material.js');
const { success, skip } = require('../../src/result.js');
const { outputResult } = require('../../src/output.js');
const { logInfo, logWarn, logError } = require('../../src/logging.js');
const { run } = require('../../src/execute.js');
const { discover } = require('../../src/discover.js');

/**
 * Main policy execution function.
 * This is the entry point called by the Chainloop engine.
 */
function Execute() {
  return run(() => {
    // Parse container image material to get the digest
    const input = getMaterialJSON();

    if (!input || !input.chainloop_metadata || !input.chainloop_metadata.digest || !input.chainloop_metadata.digest.sha256) {
      outputResult(skip("no digest found in chainloop_metadata"));
      return;
    }

    // Construct full digest with sha256 prefix
    const digest = `sha256:${input.chainloop_metadata.digest.sha256}`;
    logInfo(`Discovering artifacts related to: ${digest}`);

    // Call the discover function to explore the artifact graph
    let discoverResult;
    try {
      discoverResult = discover(digest, "");
    } catch (err) {
      logError(`Discovery failed: ${err.message}`);
      outputResult(skip(err.message));
      return;
    }

    if (!discoverResult) {
      logWarn("No discover result returned (gRPC connection may not be configured)");
      outputResult(skip("discover not available"));
      return;
    }

    // Check for policy violations in related attestations
    const result = checkAttestationViolations(discoverResult);

    // Output result
    outputResult(result);
  });
}

/**
 * Checks if any related attestations have policy violations.
 *
 * @param {Object} discoverResult - The result from the discover function
 * @returns {Object} The validation result
 */
function checkAttestationViolations(discoverResult) {
  const result = success();
  const references = discoverResult.references || [];

  logInfo(`Found ${references.length} references for artifact ${discoverResult.digest}`);

  // Check each reference for attestations with policy violations
  for (const ref of references) {
    // Only check attestations
    if (ref.kind !== "ATTESTATION") {
      continue;
    }

    logInfo(`Checking attestation: ${ref.digest}`);

    // Check if this attestation has policy violations
    if (ref.metadata && ref.metadata.hasPolicyViolations === "true") {
      // Extract metadata for detailed violation message
      const name = ref.metadata.name || "";
      const project = ref.metadata.project || "";
      const organization = ref.metadata.organization || "";

      const msg = `attestation with digest ${ref.digest} contains policy violations [name: ${name}, project: ${project}, org: ${organization}]`;
      result.addViolation(msg);
      logError(msg);
    }
  }

  // Log summary
  if (result.hasViolations()) {
    logError("Validation failed: found attestations with policy violations");
  } else {
    logInfo("Validation passed: no related attestations have policy violations");
  }

  return result;
}

module.exports = { Execute };
