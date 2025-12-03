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
 * Logs an informational message.
 * Visible in CLI output at INFO level and above.
 *
 * @param {string} message - The message to log
 *
 * @example
 * logInfo("Processing 42 components");
 */
function logInfo(message) {
  console.info(message);
}

/**
 * Logs a debug message.
 * Visible in CLI output with --debug flag.
 *
 * @param {string} message - The message to log
 *
 * @example
 * logDebug("Component details: " + JSON.stringify(component));
 */
function logDebug(message) {
  console.debug(message);
}

/**
 * Logs a warning message.
 * Visible in CLI output at WARN level and above.
 *
 * @param {string} message - The message to log
 *
 * @example
 * logWarn("Missing optional field: description");
 */
function logWarn(message) {
  console.warn(message);
}

/**
 * Logs an error message.
 * Visible in CLI output at ERROR level and above.
 *
 * @param {string} message - The message to log
 *
 * @example
 * logError("Failed to validate package: " + packageName);
 */
function logError(message) {
  console.error(message);
}

module.exports = {
  logInfo,
  logDebug,
  logWarn,
  logError
};
