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
// MERCHANTABILITY under A PARTICULAR PURPOSE.  See the
// License for the specific language governing permissions and
// limitations under the License.

/**
 * Artifact Discovery Functions
 *
 * Functions for exploring the artifact graph and discovering related artifacts.
 *
 * NOTE: Host and Memory are global objects provided by the Extism runtime.
 * They do NOT need to be imported from '@extism/js-pdk'.
 */

/**
 * Discover calls the Chainloop discover builtin to explore the artifact graph.
 * It retrieves information about artifacts related to the given digest.
 *
 * @param {string} digest - The artifact digest to discover (e.g., "sha256:abc123...")
 * @param {string} [kind=''] - Optional filter by material kind (e.g., "CONTAINER_IMAGE", "ATTESTATION")
 * @returns {Object} Information about the discovered artifact and its references
 * @throws {Error} If the discovery fails
 *
 * @example
 * // Discover all references for a container image
 * const digest = "sha256:abc123...";
 * try {
 *   const result = discover(digest);
 *
 *   // Check if any referenced attestations have policy violations
 *   for (const ref of result.references) {
 *     if (ref.kind === "ATTESTATION") {
 *       if (ref.metadata.hasPolicyViolations === "true") {
 *         console.warn(`Attestation ${ref.digest} has policy violations`);
 *       }
 *     }
 *   }
 * } catch (err) {
 *   console.error(`Discovery failed: ${err.message}`);
 * }
 *
 * @example
 * // Discover with kind filter
 * const result = discover(digest, "ATTESTATION");
 */
function discover(digest, kind = '') {
    const { chainloop_discover } = Host.getFunctions();

    const digestMem = Memory.fromString(digest);
    const kindMem = Memory.fromString(kind);

    const resultOffset = chainloop_discover(digestMem.offset, kindMem.offset);

    if (resultOffset === 0n) {
        throw new Error('discover returned error (check if gRPC connection is configured)');
    }

    // Read the result from memory using Memory.find()
    const resultMem = Memory.find(resultOffset);
    if (!resultMem) {
        throw new Error('failed to read discover result from memory');
    }

    try {
        return resultMem.readJsonObject();
    } catch (e) {
        throw new Error(`failed to parse discover result: ${e.message}`);
    }
}

/**
 * DiscoverByDigest is a convenience function that calls discover with no kind filter.
 * It retrieves all artifacts related to the given digest regardless of their type.
 *
 * @param {string} digest - The artifact digest to discover
 * @returns {Object} Information about the discovered artifact and its references
 * @throws {Error} If the discovery fails
 *
 * @example
 * const result = discoverByDigest("sha256:abc123...");
 */
function discoverByDigest(digest) {
  return discover(digest, '');
}

module.exports = {
  discover,
  discoverByDigest
};
