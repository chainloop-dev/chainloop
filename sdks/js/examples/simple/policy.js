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
 * Simple policy example that validates basic string input.
 *
 * This example demonstrates:
 * - Using run() wrapper to handle errors gracefully
 * - Material extraction using getMaterialJSON()
 * - Args extraction for configuration
 * - Result building with helper methods
 * - Logging with logInfo(), logError()
 *
 * Build:
 *   npm install
 *   npm run build
 *
 * Test:
 *   echo '{"message": "hello"}' > /tmp/test-message.json
 *   chainloop policy develop eval \
 *     --policy policy.yaml \
 *     --material /tmp/test-message.json \
 *     --kind EVIDENCE
 */

const {
  getMaterialJSON,
  getArgStringDefault,
  success,
  skip,
  outputResult,
  logInfo,
  logError,
  run
} = require('../../index.js');

/**
 * Main policy execution function.
 * This is the entry point called by the Chainloop engine.
 */
function Execute() {
  return run(() => {
    // Get max length from args, default to 100
    let maxLength = 100;
    const maxLengthStr = getArgStringDefault("max_length", "100");
    maxLength = parseInt(maxLengthStr, 10);

    logInfo(`Validating message with max length: ${maxLength}`);

    // Parse material
    const input = getMaterialJSON();

    // Check if material has the expected structure
    if (!input || typeof input.message === 'undefined') {
      outputResult(skip("Material missing 'message' field"));
      return;
    }

    // Validate
    const result = validateMessage(input, maxLength);

    // Output result
    outputResult(result);
  });
}

/**
 * Validates the message for compliance.
 *
 * @param {Object} input - The input object with message field
 * @param {number} maxLength - Maximum allowed message length
 * @returns {Object} The validation result
 */
function validateMessage(input, maxLength) {
  const result = success();
  const message = input.message;

  // Validation 1: Message must not be empty
  if (message === "") {
    result.addViolation("message cannot be empty");
    return result;
  }

  // Validation 2: Message must not contain forbidden words
  const forbidden = ["forbidden", "banned", "prohibited"];
  const messageLower = message.toLowerCase();

  for (const word of forbidden) {
    if (messageLower.includes(word)) {
      result.addViolation(`message contains forbidden word: ${word}`);
    }
  }

  // Validation 3: Message must not be too long
  if (message.length > maxLength) {
    result.addViolation(`message too long: ${message.length} characters (max ${maxLength})`);
  }

  if (result.hasViolations()) {
    logError(`Validation failed with ${result.violations.length} violations`);
  } else {
    logInfo(`Validation passed for message: ${message}`);
  }

  return result;
}

module.exports = { Execute };
