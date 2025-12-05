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
 * WebAssembly module interface for the discover policy example.
 * This file is required by extism-js compiler to generate proper WASM exports.
 */

/**
 * Host functions provided by Chainloop runtime.
 * These functions are implemented in the host (Chainloop engine) and callable from WASM.
 */
declare module "extism:host" {
// declare module "env" {
    interface user {
        /**
         * Discover builtin function for exploring the artifact graph.
         * @param digestOffset - Memory offset of the digest string
         * @param kindOffset - Memory offset of the kind string (optional, can be 0)
         * @returns Memory offset of the JSON result, or 0 on error
         */
        chainloop_discover(digestOffset: I64, kindOffset: I64): I64;
    }
}

declare module "main" {
  /**
   * Main policy execution function exported to WASM.
   * @returns Exit code (0 for success, 1 for error)
   */
  export function Execute(): I32;
}
