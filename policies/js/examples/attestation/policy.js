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

// Attestation policy that validates in-toto attestations for git commits.
//
// This example demonstrates:
// - Using run() wrapper for clean API
// - Parsing in-toto attestation materials
// - Complex validation logic with nested structs
// - Logging validation details
//
// Build:
//   npm run build
//
// Test:
//   chainloop policy develop eval \
//     --policy policy.yaml \
//     --material attestation.json \
//     --kind ATTESTATION

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
    const attestation = getMaterialJSON();

    logInfo(`Validating in-toto attestation with ${attestation.subject?.length || 0} subjects`);

    // Skip if not in-toto attestation
    if (attestation._type !== "https://in-toto.io/Statement/v0.1" &&
        attestation._type !== "https://in-toto.io/Statement/v1") {
      outputResult(skip("not an in-toto attestation"));
      return;
    }

    // Validate
    const result = validateAttestation(attestation);

    if (result.hasViolations()) {
      logError(`Attestation validation failed with ${result.violations.length} violations`);
    } else {
      logInfo(`Attestation validation passed`);
    }

    // Output result
    outputResult(result);
  });
}

// validateAttestation checks the attestation for compliance.
function validateAttestation(attestation) {
  const result = success();

  // Check subjects exist
  if (!attestation.subject || attestation.subject.length === 0) {
    result.addViolation("attestation must contain at least one subject");
    return result;
  }

  // Check for git commit subject
  let hasGitCommit = false;
  for (const subject of attestation.subject) {
    if (subject.name === "git.head") {
      hasGitCommit = true;
      logDebug("Found git.head subject");

      // Check if git commit has SHA1 digest
      if (!subject.digest || !subject.digest.sha1) {
        result.addViolation("git.head subject missing sha1 digest");
      } else {
        const sha1 = subject.digest.sha1;
        if (sha1 === "") {
          result.addViolation("git.head subject has empty sha1 digest");
        } else if (sha1.length !== 40) {
          result.addViolation(`git.head sha1 digest has invalid length: ${sha1.length} (expected 40)`);
        } else if (!isValidHex(sha1)) {
          result.addViolation("git.head sha1 digest contains invalid characters");
        } else {
          logDebug(`Valid git commit SHA1: ${sha1}`);
        }
      }
    }
  }

  if (!hasGitCommit) {
    result.addViolation("attestation must reference a git commit (git.head)");
  }

  // Check predicate type is not empty
  if (!attestation.predicateType || attestation.predicateType === "") {
    result.addViolation("attestation must have a predicateType");
  }

  return result;
}

// isValidHex checks if a string contains only hexadecimal characters.
function isValidHex(s) {
  const lowerS = s.toLowerCase();
  for (let i = 0; i < lowerS.length; i++) {
    const c = lowerS[i];
    if (!((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'))) {
      return false;
    }
  }
  return true;
}

module.exports = { Execute };
