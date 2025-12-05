# Makefile for {{.Name}} WASM policy
#
# Copyright 2025 The Chainloop Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.PHONY: build clean lint help

# Default target
.DEFAULT_GOAL := help

# Build the WASM policy
build: ## Build the WASM policy
	@echo "Building WASM policy..."
	tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
	@echo "✓ Built policy.wasm"

# Clean build artifacts
clean: ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	rm -f policy.wasm
	@echo "Cleaned"

# Download dependencies
deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	go mod download
	@echo "Dependencies downloaded"

# Format Go code
fmt: ## Format Go code
	@echo "Formatting Go code..."
	gofmt -w .
	@echo "Code formatted"

# Show help
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
