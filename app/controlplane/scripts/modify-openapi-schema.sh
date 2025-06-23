#!/usr/bin/env bash

set -euo pipefail

# Usage: ./modify-openapi-schema.sh <host-mount-dir> <input-swagger-json-relative-to-mount> <output-openapi-yaml-relative-to-mount> [<base-openapi-yaml-relative-to-mount>]
# Example: ./modify-openapi-schema.sh ./gen temp-openapi/apidocs.swagger.json openapi/openapi/openapi.yaml openapi/base-openapi.yaml

if [ $# -lt 3 ] || [ $# -gt 4 ]; then
  echo "Usage: $0 <host-mount-dir> <input-swagger-json-relative-to-mount> <output-openapi-yaml-relative-to-mount> [<base-openapi-yaml-relative-to-mount>]"
  exit 1
fi

HOST_MOUNT_DIR="$1"
INPUT_SWAGGER_JSON="$2"
OUTPUT_OPENAPI_YAML="$3"
BASE_OPENAPI_YAML="${4:-}"  # Optional 4th parameter for base document

# Check for required tools and Docker daemon
if ! command -v yq >/dev/null 2>&1; then
  echo "yq is not installed. Please install yq by running 'make init'" >&2
  exit 2
elif ! command -v docker >/dev/null 2>&1; then
  echo "Docker is not installed or not in PATH." >&2
  exit 2
elif ! docker info >/dev/null 2>&1; then
  echo "Docker is not running or you do not have permission to access the Docker daemon." >&2
  exit 2
fi

# Modify the security scheme in the generated OpenAPI YAML
modify_security_scheme() {
  local openapi_file="$1"
  yq -i '
    .components.securitySchemes.bearerToken.type = "http" |
    .components.securitySchemes.bearerToken.scheme = "bearer" |
    .components.securitySchemes.bearerToken.bearerFormat = "JWT" |
    del(.components.securitySchemes.bearerToken.name) |
    del(.components.securitySchemes.bearerToken.in)
  ' "$openapi_file"
}

# Clean up temporary files and directories
cleanup() {
  local host_mount_dir="$1"

  # Clean up temp directory inside the mounted host directory
  rm -rf "$host_mount_dir/temp-openapi"

  # Clean up the generated OpenAPI supporting files
  rm -rf "$host_mount_dir/openapi/.openapi-generator" "$host_mount_dir/openapi/.openapi-generator-ignore" "$host_mount_dir/openapi/README.md"

  # Move up the generated OpenAPI YAML file
  cp "$host_mount_dir/openapi/openapi/openapi.yaml" "$host_mount_dir/openapi/"

  # Remove the now-empty directory
  rm -rf "$host_mount_dir/openapi/openapi"
}

# Run OpenAPI Generator CLI via Docker, mounting the specified host directory
docker run --rm --user "$(id -u)":"$(id -g)" \
  -v "$(realpath "$HOST_MOUNT_DIR"):/local" \
  openapitools/openapi-generator-cli@sha256:a711d89180b9ce34348413a830b21e9c4d3bdf325a154659211cd7a737b0f95a generate \
    -i "/local/$INPUT_SWAGGER_JSON" \
    -g openapi-yaml \
    -o "/local/openapi"

modify_security_scheme "$HOST_MOUNT_DIR/$OUTPUT_OPENAPI_YAML"

# Merge with base document if provided, otherwise merge with openapi-extra.yaml
if [ -n "$BASE_OPENAPI_YAML" ]; then
  echo "Merging with base document: $BASE_OPENAPI_YAML"

  # Check if the base document exists
  if [ ! -f "$HOST_MOUNT_DIR/$BASE_OPENAPI_YAML" ]; then
    echo "Base OpenAPI file not found: $HOST_MOUNT_DIR/$BASE_OPENAPI_YAML" >&2
    exit 3
  fi

  echo "Base document exists at: $HOST_MOUNT_DIR/$BASE_OPENAPI_YAML"

  # Run Redocly CLI to join the specs
  docker run --rm -v "$(realpath "$HOST_MOUNT_DIR"):/spec" \
    redocly/cli@sha256:a2e50da1c3807122c9d2e0d2a83e11ddc1c60b676b50d08b02c5dde8506f3eee join \
    "/spec/$OUTPUT_OPENAPI_YAML" \
    "/spec/$BASE_OPENAPI_YAML" \
    -o "/spec/$OUTPUT_OPENAPI_YAML" \
    --without-x-tag-groups

  if [ $? -ne 0 ]; then
    echo "Failed to merge OpenAPI specifications" >&2
    exit 4
  fi

  echo "Successfully merged OpenAPI specifications"
fi

cleanup "$HOST_MOUNT_DIR"

