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
PUBLIC_KEY_URL="https://raw.githubusercontent.com/chainloop-dev/chainloop/01ad13af08950b7bfbc83569bea207aeb4e1a285/docs/static/cosign-releases.pub"

# Constants
GITHUB_BASE_URL="https://github.com/chainloop-dev/chainloop/releases/download"
GITHUB_LATEST_RELEASE_URL="https://github.com/chainloop-dev/chainloop/releases/latest"

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
  grep "$FILENAME" "$2" > checksum.txt
  if is_command sha256sum; then
    sha256sum -c checksum.txt
  elif is_command shasum; then
    shasum -a 256 -q -c checksum.txt
  else
    fancy_print 1 "We were not able to verify checksums. Commands sha256sum, shasum are not found."
    return 1
  fi
  fancy_print 2 "Checksum OK\n"
}

# Check legacy installations downloads and inspects the checksum.txt file
# New Chainloop releases does not include the .tar.gz files for the CLI anymore
# and instead provides a single binary file for each OS and architecture that can be downloaded
# directly.
check_if_legacy_installation() {
    local TMP_DIR=$1
    local CHECKSUM_FILE="${TMP_DIR}/checksums.txt"
    local URL="${GITHUB_BASE_URL}/v${VERSION}/checksums.txt"

    CHECKSUM_FILENAME=checksums.txt
    CHECKSUM_FILE="$TMP_DIR/${CHECKSUM_FILENAME}"
    curl -fsL "$URL" -o "${CHECKSUM_FILE}" || (fancy_print 1 "The requested file does not exist: ${URL}"; exit 1)

    grep -q "chainloop-cli-${VERSION}-${OS}-${ARC}.tar.gz" "$CHECKSUM_FILE" && echo "true" || echo "false"
}

# Get the latest version from the GitHub releases page
get_latest_version() {
    curl -sI -o /dev/null -w '%{redirect_url}' "$GITHUB_LATEST_RELEASE_URL" | sed -n 's#.*/tag/\(v.*\)#\1#p'
}

# Download the checksum file and verify it
download_and_check_checksum() {
    local TMP_DIR=$1
    local BASE_URL=$2

    CHECKSUM_FILENAME=checksums.txt
    CHECKSUM_FILE="$TMP_DIR/${CHECKSUM_FILENAME}"
    URL="$BASE_URL/${CHECKSUM_FILENAME}"
    curl -fsL "$URL" -o "${CHECKSUM_FILE}" || (fancy_print 1 "The requested file does not exist: ${URL}"; exit 1)
    validate_checksums_file "${TMP_DIR}" checksums.txt

    # Verify checksum file signature
    if hash "cosign" &>/dev/null; then
        # Constructing download FILE and URL
        SIGNATURE_FILE="${CHECKSUM_FILENAME}.sig"
        URL="$BASE_URL/${SIGNATURE_FILE}"
        # Download file, exit if not found - e.g. version does not exist
        fancy_print 0 "Step 1.3: Verifying signature"
        curl -fsOL "$URL" || (fancy_print 1 "The requested file does not exist: ${SIGNATURE_FILE}"; exit 1)
        cosign verify-blob --key ${PUBLIC_KEY_URL} --signature ${SIGNATURE_FILE} "${CHECKSUM_FILE}"

        rm $SIGNATURE_FILE
    else
        fancy_print 2 "\nSignature verification skipped, cosign is not installed\n"
    fi
}

cleanup() {
    local tmp_dir=$1
    rm -rf "$tmp_dir"
    fancy_print 0 "Done...\n"
}

install_binary() {
    local binary_path=$1
    install "$binary_path" "${INSTALL_PATH}/" 2>/dev/null || sudo install "$binary_path" "${INSTALL_PATH}/"
}

post_install_message() {
    "${INSTALL_PATH}/chainloop" version
    fancy_print 2 "Check here for the next steps: https://docs.chainloop.dev\n"
    fancy_print 2 "Run 'chainloop auth login' to get started"
}

download_and_install_legacy() {
  local TMP_DIR=$1
  FILENAME="chainloop-cli-${VERSION}-${OS}-${ARC}.tar.gz"
  # Constructing download FILE and URL
  FILE="$TMP_DIR/${FILENAME}"

  BASE_URL="${GITHUB_BASE_URL}/v${VERSION}"

  URL="${BASE_URL}/${FILENAME}"
  # Download file, exit if not found - e.g. version does not exist
  fancy_print 0 "Step 1: Downloading: ${FILENAME}"
  curl -fsL "$URL" -o "$FILE" || (fancy_print 1 "The requested file does not exist: ${URL}"; exit 1)
  fancy_print 0 "Done...\n"

  # Get checksum file and check it
  fancy_print 0 "Step 1.2: Verifying checksum"
  download_and_check_checksum "$TMP_DIR" "$BASE_URL"

  # Decompress the file
  fancy_print 0 "Step 2: Decompressing: ${FILE}"
  (cd "${TMP_DIR}" && tar xf "$FILE")
  fancy_print 0 "Done...\n"

  # Install
  fancy_print 0 "Step 3: Installing: chainloop in path ${INSTALL_PATH}"
  install_binary "${TMP_DIR}/chainloop"

  # Remove the compressed file
  fancy_print 0 "Step 4: Cleanup"
  cleanup "$TMP_DIR"

  post_install_message
}

download_and_install() {
    local TMP_DIR=$1
    BASE_URL="${GITHUB_BASE_URL}/v${VERSION}"

    FILENAME="chainloop-${OS}-${ARC}"
    # Constructing download FILE and URL
    FILE="$TMP_DIR/${FILENAME}"

    URL="${BASE_URL}/${FILENAME}"
    # Download file, exit if not found - e.g. version does not exist
    fancy_print 0 "Step 1: Downloading: ${FILENAME}, Version: ${VERSION}"
    curl -fsL "$URL" -o "$FILE" || (fancy_print 1 "The requested file does not exist: ${URL}"; exit 1)
    fancy_print 0 "Done...\n"

    # Get checksum file and check it
    fancy_print 0 "Step 1.2: Verifying checksum"
    download_and_check_checksum "$TMP_DIR" "$BASE_URL"

    # Modify the name of the binary
    # From chainloop-OS-ARCH to chainloop
    cp "${FILE}" "chainloop"

    # Install
    fancy_print 0 "Step 2: Installing: chainloop to ${INSTALL_PATH}"
    install_binary "${TMP_DIR}/chainloop"

    fancy_print 0 "Step 3: Cleanup"
    cleanup "$TMP_DIR"

    post_install_message
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
# Remove v prefix
VERSION="${VERSION:-$(get_latest_version | sed 's/^v//')}"

# Temporary directory, works on Linux and macOS
TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'mytmpdir')

# Decide which method to use to install Chainloop
if [[ $(check_if_legacy_installation "$TMP_DIR") == "true" ]]; then
    download_and_install_legacy "$TMP_DIR"
else
    download_and_install "$TMP_DIR"
fi
