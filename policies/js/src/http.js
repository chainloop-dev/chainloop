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
 * Performs an HTTP GET request.
 * Only hostnames configured in the engine's allowed list are accessible.
 *
 * @param {string} url - The URL to request
 * @returns {Object} Response object with {status: number, body: string}
 * @throws {Error} If the request fails or hostname is not allowed
 *
 * @example
 * const response = httpGet("https://registry.npmjs.org/lodash");
 * if (response.status === 200) {
 *   const pkg = JSON.parse(response.body);
 *   console.log(`Package: ${pkg.name}`);
 * }
 */
function httpGet(url) {
  const request = {
    method: "GET",
    url: url
  };

  try {
    const response = Http.request(request);
    return {
      status: response.status,
      body: response.body
    };
  } catch (e) {
    throw new Error(`HTTP GET failed for ${url}: ${e.message}`);
  }
}

/**
 * Performs an HTTP GET request and parses the response as JSON.
 *
 * @param {string} url - The URL to request
 * @returns {Object} The parsed JSON response
 * @throws {Error} If the request fails or response is not valid JSON
 *
 * @example
 * const pkg = httpGetJSON("https://registry.npmjs.org/lodash");
 * console.log(`Latest version: ${pkg['dist-tags'].latest}`);
 */
function httpGetJSON(url) {
  const response = httpGet(url);

  if (response.status !== 200) {
    throw new Error(`HTTP request failed with status ${response.status}`);
  }

  try {
    return JSON.parse(response.body);
  } catch (e) {
    throw new Error(`Failed to parse response as JSON: ${e.message}`);
  }
}

/**
 * Performs an HTTP POST request.
 *
 * @param {string} url - The URL to request
 * @param {string} body - The request body
 * @returns {Object} Response object with {status: number, body: string}
 * @throws {Error} If the request fails
 *
 * @example
 * const response = httpPost("https://api.example.com/validate", jsonData);
 */
function httpPost(url, body) {
  const request = {
    method: "POST",
    url: url,
    body: body
  };

  try {
    const response = Http.request(request);
    return {
      status: response.status,
      body: response.body
    };
  } catch (e) {
    throw new Error(`HTTP POST failed for ${url}: ${e.message}`);
  }
}

/**
 * Performs an HTTP POST request with JSON body and response.
 *
 * @param {string} url - The URL to request
 * @param {Object} requestBody - The object to send as JSON
 * @returns {Object} The parsed JSON response
 * @throws {Error} If the request fails or response is not valid JSON
 *
 * @example
 * const result = httpPostJSON("https://api.example.com/validate", {data: "test"});
 */
function httpPostJSON(url, requestBody) {
  const body = JSON.stringify(requestBody);
  const response = httpPost(url, body);

  if (response.status !== 200) {
    throw new Error(`HTTP request failed with status ${response.status}`);
  }

  try {
    return JSON.parse(response.body);
  } catch (e) {
    throw new Error(`Failed to parse response as JSON: ${e.message}`);
  }
}

module.exports = {
  httpGet,
  httpGetJSON,
  httpPost,
  httpPostJSON
};
