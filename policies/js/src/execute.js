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

const { logError } = require('./logging');
const { outputResult, fail } = require('./output');

/**
 * Wrapper function for policy execution that handles errors gracefully.
 *
 * @param {Function} fn - The policy function to execute
 * @returns {number} Exit code (0 for success, 1 for error)
 *
 * @example
 * function Execute() {
 *   return run(() => {
 *     const material = getMaterialJSON();
 *     const result = success();
 *     // ... validation logic ...
 *     outputResult(result);
 *   });
 * }
 */
function run(fn) {
  try {
    fn();
    return 0;
  } catch (e) {
    logError(`Policy execution failed: ${e.message}`);
    outputResult(fail(`Policy execution error: ${e.message}`));
    return 1;
  }
}

module.exports = {
  run
};
