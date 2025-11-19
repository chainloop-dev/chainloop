#!/usr/bin/env bash

# Script to add license information to SBOM components
# Usage: 
#   ./add-license-to-sbom.sh <sbom_file> <component_name> <license_identifier> [version=<version>] [type=<type>] [--custom_license] [--strict] [--help]
#   Example: ./add-license-to-sbom.sh sbom.json "backend" "Apache-2.0" version="v1.0.0"
#   Example: ./add-license-to-sbom.sh sbom.json "frontend" "Chainloop Proprietary License" type="library" --custom_license --strict

set -euo pipefail

# Check for help flag first
if [[ "${1:-}" == "--help" ]] || [[ "${1:-}" == "-h" ]]; then
  cat << EOF
SBOM License Addition Script

This script adds license information to SBOM components if they exist and don't already have licenses.

Usage:
  ./add-license-to-sbom.sh <sbom_file> <component_name> <license_identifier> [options]

Arguments:
  sbom_file          Path to the SBOM file (CycloneDX JSON format)
  component_name     Name of the component to add license to
  license_identifier SPDX license identifier (e.g., "Apache-2.0", "MIT") or custom license name

Options:
  version=<ver>      Match component by name and version
  type=<type>        Match component by name and type
  --custom_license   Treat license_identifier as custom license name (default: SPDX identifier)
  --strict           Fail if component doesn't exist or already has licenses
  --help, -h         Show this help message

Examples:
  # Add SPDX license identifier to component by name and version
  ./add-license-to-sbom.sh sbom.json "backend" "Apache-2.0" version="v1.0.0"
  
  # Add SPDX license identifier to component by name and type
  ./add-license-to-sbom.sh sbom.json "frontend" "MIT" type="library"
  
  # Add custom license name to component
  ./add-license-to-sbom.sh sbom.json "backend" "Chainloop Proprietary License" --custom_license
  
  # Use strict mode (fail if component missing or has licenses)
  ./add-license-to-sbom.sh sbom.json "cli" "BSD-3-Clause" --strict
  
  # Match by name only with SPDX identifier
  ./add-license-to-sbom.sh sbom.json "backend" "GPL-3.0-or-later"

Behavior:
  - Default: Skips with message if component not found or already has licenses
  - Strict:  Exits with error code 1 if component not found or already has licenses
EOF
  exit 0
fi

# Validate required arguments
if [[ $# -lt 3 ]]; then
  echo "Error: Missing required arguments"
  echo "Use --help for usage information"
  exit 1
fi

SBOM_FILE="$1"
COMPONENT_NAME="$2"
LICENSE_NAME="$3"

# Validate SBOM file exists
if [[ ! -f "$SBOM_FILE" ]]; then
  echo "Error: SBOM file '$SBOM_FILE' does not exist"
  exit 1
fi

# Initialize optional parameters
VERSION=""
TYPE=""
CUSTOM_LICENSE=false  # Default to SPDX identifier format
STRICT_MODE=false

# Parse optional parameters from command line arguments (starting from 4th argument)
for arg in "${@:4}"; do
  case $arg in
    version=*)
      # Extract version value after the '=' sign
      VERSION="${arg#*=}"
      ;;
    type=*)
      # Extract type value after the '=' sign
      TYPE="${arg#*=}"
      ;;
    --custom_license)
      # Treat license_identifier as custom license name instead of SPDX identifier
      CUSTOM_LICENSE=true
      ;;
    --strict)
      # Enable strict mode - will fail if component missing or has licenses
      STRICT_MODE=true
      ;;
  esac
done

# Build jq selector based on available criteria to identify the component
# This creates a flexible matching system depending on what parameters were provided
if [[ -n "$VERSION" && -n "$TYPE" ]]; then
  # Match by name, version AND type (most specific)
  SELECTOR=".name == \"$COMPONENT_NAME\" and .version == \"$VERSION\" and .type == \"$TYPE\""
  IDENTIFIER="name='$COMPONENT_NAME', version='$VERSION', type='$TYPE'"
elif [[ -n "$VERSION" ]]; then
  # Match by name and version only
  SELECTOR=".name == \"$COMPONENT_NAME\" and .version == \"$VERSION\""
  IDENTIFIER="name='$COMPONENT_NAME', version='$VERSION'"
elif [[ -n "$TYPE" ]]; then
  # Match by name and type only
  SELECTOR=".name == \"$COMPONENT_NAME\" and .type == \"$TYPE\""
  IDENTIFIER="name='$COMPONENT_NAME', type='$TYPE'"
else
  # Match by name only (least specific)
  SELECTOR=".name == \"$COMPONENT_NAME\""
  IDENTIFIER="name='$COMPONENT_NAME'"
fi

# Check if component exists in SBOM using our constructed selector
# Uses jq to search through components array and returns the name if found
HAS_COMPONENT=$(jq -r ".components[] | select($SELECTOR) | .name" "$SBOM_FILE" | head -1)

if [[ -n "$HAS_COMPONENT" ]]; then
  # Component was found - now check if it already has license information
  # Count existing licenses (using // [] to handle missing licenses field)
  HAS_LICENSES=$(jq -r ".components[] | select($SELECTOR) | (.licenses // []) | length" "$SBOM_FILE" | head -1)
  
  if [[ "$HAS_LICENSES" == "0" ]]; then
    # Component exists but has no licenses - proceed with adding license
    echo "Adding license '$LICENSE_NAME' to component with $IDENTIFIER"
    
    # Use jq to add license information to the matching component
    # Creates temporary file and atomically moves it to avoid corruption
    if [[ "$CUSTOM_LICENSE" == "true" ]]; then
      # Use "name" field for custom license names
      jq "(.components[] | select($SELECTOR) | select((.licenses // []) | length == 0) | .licenses) = [{\"license\": {\"name\": \"$LICENSE_NAME\"}}]" "$SBOM_FILE" > "${SBOM_FILE}.tmp" && mv "${SBOM_FILE}.tmp" "$SBOM_FILE"
    else
      # Use "id" field for SPDX license identifiers
      jq "(.components[] | select($SELECTOR) | select((.licenses // []) | length == 0) | .licenses) = [{\"license\": {\"id\": \"$LICENSE_NAME\"}}]" "$SBOM_FILE" > "${SBOM_FILE}.tmp" && mv "${SBOM_FILE}.tmp" "$SBOM_FILE"
    fi
    
    echo "License added successfully"
  else
    # Component already has license information
    echo "Component with $IDENTIFIER already has licenses"
    if [[ "$STRICT_MODE" == "true" ]]; then
      # In strict mode, this is an error condition
      echo "ERROR: --strict mode enabled and component already has licenses"
      exit 1
    else
      # In lenient mode, just skip with a message
      echo "Skipping license addition"
    fi
  fi
else
  # Component was not found in SBOM
  echo "Component with $IDENTIFIER not found in SBOM"
  if [[ "$STRICT_MODE" == "true" ]]; then
    # In strict mode, missing component is an error
    echo "ERROR: --strict mode enabled and component not found"
    exit 1
  else
    # In lenient mode, just skip with a message
    echo "Skipping license addition"
  fi
fi