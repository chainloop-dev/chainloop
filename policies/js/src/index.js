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
 * Chainloop Policy SDK for JavaScript
 *
 * High-level SDK for writing Chainloop WASM policies in JavaScript/TypeScript.
 * Built on top of the Extism JS PDK.
 *
 * @module chainloop-policy-sdk
 */

// Import all functions from separate modules
const {
  getMaterialJSON,
  getMaterialString,
  getMaterialBytes
} = require('./material');

const {
  getArgs,
  getArgString,
  getArgStringDefault
} = require('./args');

const {
  logInfo,
  logDebug,
  logWarn,
  logError
} = require('./logging');

const {
  httpGet,
  httpGetJSON,
  httpPost,
  httpPostJSON
} = require('./http');

const {
  success,
  fail,
  skip
} = require('./result');

const {
  outputResult
} = require('./output');

const {
  run
} = require('./execute');

const {
  discover,
  discoverByDigest
} = require('./discover');

// Re-export all functions
module.exports = {
  // Material extraction
  getMaterialJSON,
  getMaterialString,
  getMaterialBytes,

  // Arguments
  getArgs,
  getArgString,
  getArgStringDefault,

  // Logging
  logInfo,
  logDebug,
  logWarn,
  logError,

  // HTTP
  httpGet,
  httpGetJSON,
  httpPost,
  httpPostJSON,

  // Artifact Discovery
  discover,
  discoverByDigest,

  // Results
  success,
  fail,
  skip,
  outputResult,

  // Execution
  run
};
