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
 * Creates a successful result (no violations, not skipped).
 *
 * @returns {Object} A success result
 *
 * @example
 * const result = success();
 * result.addViolation("Found an issue");
 * outputResult(result);
 */
function success() {
  return {
    skipped: false,
    violations: [],
    skip_reason: "",
    ignore: false,

    /**
     * Adds a violation message to the result.
     * @param {string} message - The violation message
     */
    addViolation(message) {
      this.violations.push(message);
    },

    /**
     * Returns true if there are any violations.
     * @returns {boolean}
     */
    hasViolations() {
      return this.violations.length > 0;
    },

    /**
     * Returns true if the policy passed (no violations, not skipped).
     * @returns {boolean}
     */
    isSuccess() {
      return !this.skipped && this.violations.length === 0;
    }
  };
}

/**
 * Creates a failed result with one or more violations.
 *
 * @param {...string} violations - One or more violation messages
 * @returns {Object} A failure result
 *
 * @example
 * const result = fail("Missing field: name", "Invalid version format");
 * outputResult(result);
 */
function fail(...violations) {
  return {
    skipped: false,
    violations: violations,
    skip_reason: "",
    ignore: false,

    addViolation(message) {
      this.violations.push(message);
    },

    hasViolations() {
      return this.violations.length > 0;
    },

    isSuccess() {
      return false;
    }
  };
}

/**
 * Creates a skipped result with a reason.
 * Use this when the policy doesn't apply to the material.
 *
 * @param {string} reason - The reason for skipping
 * @returns {Object} A skip result
 *
 * @example
 * const result = skip("Not a CycloneDX SBOM");
 * outputResult(result);
 */
function skip(reason) {
  return {
    skipped: true,
    violations: [],
    skip_reason: reason,
    ignore: false,

    addViolation(message) {
      this.violations.push(message);
    },

    hasViolations() {
      return this.violations.length > 0;
    },

    isSuccess() {
      return false;
    }
  };
}

module.exports = {
  success,
  fail,
  skip
};
