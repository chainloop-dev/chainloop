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

// SBOM policy example that validates CycloneDX BOMs.
//
// This example demonstrates:
// - Using run() wrapper for clean API
// - Parsing CycloneDX SBOM materials
// - Logging validation progress
// - Result building with violations
//
// Build:
//   npm run build
//
// Test:
//   chainloop policy develop eval \
//     --policy policy.yaml \
//     --material sbom.json \
//     --kind SBOM_CYCLONEDX_JSON

const {
  getMaterialJSON,
  skip,
  success,
  outputResult,
  logInfo,
  logError,
  logDebug,
  run
} = require('../../index.js');

function Execute() {
  return run(() => {
    // Parse material
    const sbom = getMaterialJSON();

    logInfo(`Validating CycloneDX SBOM with ${sbom.components?.length || 0} components`);

    // Skip if not CycloneDX
    if (sbom.bomFormat !== "CycloneDX") {
      outputResult(skip("not a CycloneDX SBOM"));
      return;
    }

    // Validate
    const result = validateSBOM(sbom);

    if (result.hasViolations()) {
      logError(`SBOM validation failed with ${result.violations.length} violations`);
    } else {
      logInfo(`SBOM validation passed`);
    }

    // Output result
    outputResult(result);
  });
}

// validateSBOM checks the SBOM for compliance.
function validateSBOM(sbom) {
  const result = success();

  // Check components exist
  if (!sbom.components || sbom.components.length === 0) {
    result.addViolation("SBOM must contain at least one component");
    return result;
  }

  // Validate each component
  for (let i = 0; i < sbom.components.length; i++) {
    const comp = sbom.components[i];
    if (!comp.name || comp.name === "") {
      result.addViolation(`component at index ${i} missing name`);
    }
    if (!comp.version || comp.version === "") {
      result.addViolation(`component '${comp.name}' missing version`);
    }
  }

  return result;
}

module.exports = { Execute };
