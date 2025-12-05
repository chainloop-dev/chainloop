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

const { success, fail, skip } = require('./result');

/**
 * Outputs a policy result as JSON.
 * This should be called at the end of policy execution.
 *
 * @param {Object} result - The result object (from success(), fail(), or skip())
 *
 * @example
 * const result = success();
 * if (sbom.components.length === 0) {
 *   result.addViolation("SBOM must have components");
 * }
 * outputResult(result);
 */
function outputResult(result) {
  // Convert result to plain object for JSON serialization
  const output = {
    skipped: result.skipped,
    violations: result.violations,
    skip_reason: result.skip_reason,
    ignore: result.ignore
  };

  Host.outputString(JSON.stringify(output));
}

module.exports = {
  outputResult,
  // Re-export result builders for convenience
  success,
  fail,
  skip
};
