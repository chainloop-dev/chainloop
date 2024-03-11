#!/usr/bin/env bash
set -euo pipefail

# Based on https://developer.fermyon.com/ install script thanks!
# Fancy colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color aka reset

# Version to install. Defaults to latest or set by --version or -v
VERSION=""
FORCE_VERIFICATION=false
INSTALL_PATH=/usr/local/bin
PUBLIC_KEY_URL="https://raw.githubusercontent.com/chainloop-dev/docs/c5b7b9be051d8d3b9f48153c6e3ccf569e6990e8/static/cosign-releases.pub"

# Print in colors - 0=green, 1=red, 2=neutral
# e.g. fancy_print 0 "All is great"
fancy_print() {
    if [[ $1 == 0 ]]; then
        echo -e "${GREEN}${2}${NC}"
    elif [[ $1 == 1 ]]; then
        echo -e "${RED}${2}${NC}"
    else
        echo -e "${2}"
    fi
}


# Function to print the help message
print_help() {
    fancy_print 2 ""
    fancy_print 2 "---- Chainloop Installer Script ----"
    fancy_print 2 "This script installs Chainloop in the current directory."
    fancy_print 2 ""
    fancy_print 2 "Command line arguments"
    fancy_print 2 "--version or -v       Provide what version to install e.g. \"v0.5.0\"."
    fancy_print 2 "--path                Installation path (default: /usr/local/bin)"
    fancy_print 2 "--force-verification  Force signature verification of the binary with cosign."
    fancy_print 2 "--help or -h          Shows this help message"
}

# Function used to check if utilities are available
require() {
    if ! hash "$1" &>/dev/null; then
        fancy_print 1 "'$1' not found in PATH. This is required for this script to work."
        exit 1
    fi
}

# check if a command exist
is_command() {
  command -v "$1" >/dev/null
}

# checksums.txt file validation
# example: check_sha256 "${TMP_DIR}" checksums.txt
validate_checksums_file() {
  cd "$1"
  if is_command sha256sum; then
    sha256sum --ignore-missing --quiet --check "$2"
  elif is_command shasum; then
    shasum -a 256 --ignore-missing --quiet --check checksums.txt
  else
    fancy_print 1 "We were not able to verify checksums. Commands sha256sum, shasum are not found."
    return 1
  fi
  fancy_print 2 "Checksum OK\n"
}

# Parse input arguments
while [[ $# -gt 0 ]]; do
    case $1 in
    '--version' | -v)
        shift
        if [[ $# -ne 0 ]]; then
            # Remove v prefix if provided
            VERSION="$(echo ${1} | sed -e 's/^v\(.*\)/\1/')"
        else
            fancy_print 1 "Please provide the desired version. e.g. --version v0.5.0"
            exit 0
        fi
        ;;
    '--help' | -h)
        shift
        print_help
        ;;
    '--force-verification')
        FORCE_VERIFICATION=true
        ;;
    '--path')
        shift
        INSTALL_PATH=$1
        ;;
    *)
        fancy_print 1 "Unknown argument ${1}."
        print_help
        exit 1
        ;;
    esac
    shift
done

# Check all required utilities are available
require curl
require tar
require uname

if ! hash "cosign" &>/dev/null; then
    if [[ $FORCE_VERIFICATION = true ]]; then
        fancy_print 1 "--force-verification was set but Cosign is not present. Please download it from here https://docs.sigstore.dev/cosign/installation"
        exit 1
    fi
fi

# Check if we're on a supported system and get OS and processor architecture to download the right version
UNAME_ARC=$(uname -m)

case $UNAME_ARC in
"x86_64")
    ARC="amd64"
    ;;
"arm64"|"aarch64")
    ARC="arm64"
    ;;
*)
    fancy_print 1 "The Processor type: ${UNAME_ARC} is not yet supported by Chainloop."
    exit 1
    ;;
esac

case $OSTYPE in
"linux-gnu"*)
    OS="linux"
    ;;
"darwin"*)
    OS="darwin"
    ;;
*)
    fancy_print 1 "The OSTYPE: ${OSTYPE} is not supported by this script."
    exit 1
    ;;
esac

# Check desired version. Default to latest if no desired version was requested
if [[ $VERSION = "" ]]; then
   VERSION=$(curl -so- https://github.com/chainloop-dev/chainloop/releases | grep 'href="/chainloop-dev/chainloop/releases/tag/v[0-9]*.[0-9]*.[0-9]*\"' | sed -E 's/.*\/chainloop-dev\/chainloop\/releases\/tag\/(v[0-9\.]+)".*/\1/g' | head -1)
   # Remove v prefix
   VERSION="$(echo ${VERSION} | sed -e 's/^v\(.*\)/\1/')"
fi

# Temporary directory, works on Linux and macOS
TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'mytmpdir')
FILENAME="chainloop-cli-${VERSION}-${OS}-${ARC}.tar.gz"
# Constructing download FILE and URL
FILE="${TMP_DIR}/${FILENAME}"

BASE_URL="https://github.com/chainloop-dev/chainloop/releases/download/v${VERSION}"

URL="${BASE_URL}/${FILENAME}"
# Download file, exit if not found - e.g. version does not exist
fancy_print 0 "Step 1: Downloading: ${FILENAME}"
curl -fsL $URL -o $FILE || (fancy_print 1 "The requested file does not exist: ${URL}"; exit 1)
fancy_print 0 "Done...\n"

# Get checksum file and check it
fancy_print 0 "Step 1.2: Verifying checksum"
CHECKSUM_FILENAME=checksums.txt
CHECKSUM_FILE="${TMP_DIR}/${CHECKSUM_FILENAME}"
URL="${BASE_URL}/${CHECKSUM_FILENAME}"
curl -fsL $URL -o ${CHECKSUM_FILE} || (fancy_print 1 "The requested file does not exist: ${URL}"; exit 1)
validate_checksums_file "${TMP_DIR}" checksums.txt

# Verify checksum file signature
if hash "cosign" &>/dev/null; then
    # Constructing download FILE and URL
    SIGNATURE_FILE="${CHECKSUM_FILENAME}.sig"
    URL="${BASE_URL}/${SIGNATURE_FILE}"
    # Download file, exit if not found - e.g. version does not exist
    fancy_print 0 "Step 1.3: Verifying signature"
    curl -fsOL $URL || (fancy_print 1 "The requested file does not exist: ${SIGNATURE_FILE}"; exit 1)
    cosign verify-blob \
        --key ${PUBLIC_KEY_URL} \
        --signature ${SIGNATURE_FILE} \
        ${CHECKSUM_FILE}

    rm $SIGNATURE_FILE
else
    fancy_print 2 "\nSignature verification skipped, cosign is not installed\n"
fi

# Decompress the file
fancy_print 0 "Step 2: Decompressing: ${FILE}"
BINARY_NAME="chainloop"
BINARY="${TMP_DIR}/chainloop"
(cd ${TMP_DIR} && tar xf $FILE)
fancy_print 0 "Done...\n"

# Install
fancy_print 0 "Step 3: Installing: ${BINARY_NAME} in path ${INSTALL_PATH}"
install "${BINARY}" "${INSTALL_PATH}/" 2> /dev/null || sudo install "${BINARY}" "${INSTALL_PATH}/"

# Remove the compressed file
fancy_print 0 "Step 4: Cleanup"
rm -r ${TMP_DIR}
fancy_print 0 "Done...\n"
${INSTALL_PATH}/${BINARY_NAME} version


fancy_print 2 "Check here for the next steps: https://docs.chainloop.dev\n"
fancy_print 2 "Run '${BINARY_NAME} auth login' to get started"
