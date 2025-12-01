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

// HTTP example demonstrating how policies can make external API calls
// with hostname restrictions enforced by the policy engine.
//
// This example shows:
// - httpGet() to fetch data from allowed hostnames
// - httpGetJSON() to parse JSON responses
// - Hostname blocking for security
//
// Build:
//   npm run build
//
// Test with allowed hostname:
//   chainloop policy develop eval \
//     --policy policy.yaml \
//     --material data.json \
//     --kind EVIDENCE \
//     --allowed-hostnames httpbin.org
//
// Test with blocked hostname (will fail):
//   chainloop policy develop eval \
//     --policy policy.yaml \
//     --material data.json \
//     --kind EVIDENCE \
//     --allowed-hostnames www.example.com

const {
  getMaterialJSON,
  httpGetJSON,
  skip,
  success,
  fail,
  outputResult,
  logInfo,
  logError,
  run
} = require('../../index.js');

function Execute() {
  return run(() => {
    // Parse material
    const input = getMaterialJSON();

    if (!input.check_url) {
      outputResult(skip("Material missing 'check_url' field"));
      return;
    }

    logInfo(`Making HTTP request to: ${input.check_url}`);

    // Attempt HTTP request (will be blocked if hostname not allowed)
    let apiResp;
    try {
      apiResp = httpGetJSON(input.check_url);
    } catch (err) {
      // This will fail if hostname is not in allowed list
      logError(`HTTP request failed: ${err.message}`);
      outputResult(fail(`failed to fetch data: ${err.message}`));
      return;
    }

    logInfo(`HTTP request succeeded`);

    // Validate based on API response
    const result = success();

    // httpbin.org/json returns a slideshow object
    if (apiResp.slideshow) {
      logInfo(`Received slideshow data: ${apiResp.slideshow.title || 'untitled'}`);
    } else {
      result.addViolation("API response missing expected 'slideshow' field");
    }

    outputResult(result);
  });
}

module.exports = { Execute };
