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
 * Retrieves all policy arguments from the Extism config.
 * Arguments are passed via the engine configuration and control policy behavior.
 *
 * @returns {Object} Object containing all policy arguments as key-value pairs
 * @throws {Error} If args cannot be parsed
 *
 * @example
 * const args = getArgs();
 * const threshold = args.severity_threshold || "HIGH";
 */
function getArgs() {
  const argsJSON = Config.get("args");
  if (!argsJSON) {
    return {};
  }

  try {
    return JSON.parse(argsJSON);
  } catch (e) {
    throw new Error(`Failed to parse args from config: ${e.message}`);
  }
}

/**
 * Retrieves a specific argument as a string.
 *
 * @param {string} key - The argument key
 * @returns {string|undefined} The argument value, or undefined if not found
 *
 * @example
 * const threshold = getArgString("severity_threshold");
 * if (threshold) {
 *   console.log(`Using threshold: ${threshold}`);
 * }
 */
function getArgString(key) {
  const args = getArgs();
  return args[key];
}

/**
 * Retrieves a specific argument as a string with a default value.
 *
 * @param {string} key - The argument key
 * @param {string} defaultValue - Default value if key not found
 * @returns {string} The argument value or default
 *
 * @example
 * const threshold = getArgStringDefault("severity_threshold", "HIGH");
 */
function getArgStringDefault(key, defaultValue) {
  const value = getArgString(key);
  return value !== undefined ? value : defaultValue;
}

module.exports = {
  getArgs,
  getArgString,
  getArgStringDefault
};
