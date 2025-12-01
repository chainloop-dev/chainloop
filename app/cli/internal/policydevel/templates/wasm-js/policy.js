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
 * {{.Description}}
 *
 * Build:
 *   npm install
 *   npm run build
 *
 * Test:
 *   chainloop policy develop eval --policy policy.yaml --material <your-material> --kind {{.MaterialKind}}
 */

const {
  getMaterialJSON,
  getArgs,
  success,
  skip,
  outputResult,
  logInfo,
  logError,
  run
} = require('chainloop-sdk');

/**
 * Main policy execution function.
 * This is the entry point called by the Chainloop engine.
 */
function Execute() {
  return run(() => {
    // Get policy arguments (optional configuration)
    const args = getArgs();

    logInfo('Executing policy: {{.Name}}');

    // Parse material
    const input = getMaterialJSON();

    // Check if material has the expected structure
    if (!input) {
      outputResult(skip("Material is empty or invalid"));
      return;
    }

    // Validate
    const result = validate(input, args);

    // Output result
    outputResult(result);
  });
}

/**
 * Validates the input for compliance.
 *
 * @param {Object} input - The input object to validate
 * @param {Object} args - Policy arguments
 * @returns {Object} The validation result
 */
function validate(input, args) {
  const result = success();

  // Add your validation logic here
  // Example:
  // if (!input.message || input.message === "") {
  //   result.addViolation("message cannot be empty");
  // }

  if (result.hasViolations()) {
    logError(`Validation failed with ${result.violations.length} violations`);
  } else {
    logInfo('Validation passed');
  }

  return result;
}

module.exports = { Execute };
