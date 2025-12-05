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
 * Retrieves the material as a JSON object.
 * The material is the main input data passed to the policy for validation.
 *
 * @returns {Object} The parsed JSON material
 * @throws {Error} If the material cannot be parsed as JSON
 *
 * @example
 * const sbom = getMaterialJSON();
 * console.log(`Processing ${sbom.components.length} components`);
 */
function getMaterialJSON() {
  const materialStr = Host.inputString();
  try {
    return JSON.parse(materialStr);
  } catch (e) {
    throw new Error(`Failed to parse material as JSON: ${e.message}`);
  }
}

/**
 * Retrieves the material as a raw string.
 *
 * @returns {string} The material as a string
 *
 * @example
 * const text = getMaterialString();
 * console.log(`Material length: ${text.length}`);
 */
function getMaterialString() {
  return Host.inputString();
}

/**
 * Retrieves the material as raw bytes.
 *
 * @returns {Uint8Array} The material as bytes
 *
 * @example
 * const bytes = getMaterialBytes();
 * console.log(`Material size: ${bytes.length} bytes`);
 */
function getMaterialBytes() {
  return Host.inputBytes();
}

module.exports = {
  getMaterialJSON,
  getMaterialString,
  getMaterialBytes
};
