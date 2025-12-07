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
 * Chainloop Policy SDK for JavaScript/TypeScript
 * Type definitions for policy development with Extism
 */

// Material Extraction
export function getMaterialJSON<T = any>(): T;
export function getMaterialString(): string;
export function getMaterialBytes(): Uint8Array;

// Arguments
export function getArgs(): Record<string, string>;
export function getArgString(key: string): string | undefined;
export function getArgStringDefault(key: string, defaultValue: string): string;

// Logging
export function logInfo(message: string): void;
export function logDebug(message: string): void;
export function logWarn(message: string): void;
export function logError(message: string): void;

// HTTP
export interface HttpResponse {
  status: number;
  body: string;
}

export function httpGet(url: string): HttpResponse;
export function httpGetJSON<T = any>(url: string): T;
export function httpPost(url: string, body: string): HttpResponse;
export function httpPostJSON<T = any, R = any>(url: string, requestBody: T): R;

// Artifact Discovery
export interface DiscoverReference {
  digest: string;
  kind: string;
  metadata: Record<string, string>;
}

export interface DiscoverResult {
  digest: string;
  kind: string;
  references: DiscoverReference[];
}

export function discover(digest: string, kind?: string): DiscoverResult;
export function discoverByDigest(digest: string): DiscoverResult;

// Results
export interface Result {
  skipped: boolean;
  violations: string[];
  skip_reason: string;
  ignore: boolean;

  addViolation(message: string): void;
  hasViolations(): boolean;
  isSuccess(): boolean;
}

export function success(): Result;
export function fail(...violations: string[]): Result;
export function skip(reason: string): Result;
export function outputResult(result: Result): void;

// Execution
export function run(fn: () => void): number;
