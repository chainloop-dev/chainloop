#!/usr/bin/env bash

# Bump the Chainloop project version to a specific version number in configFile (defult .chainloop.yml)

set -e

die () {
    echo >&2 "$@"
    echo "usage: bump-project-version.sh [version] [configFile]"
    exit 1
}

## debug if desired
if [[ -n "${DEBUG}" ]]; then
    set -x
fi

[ "$#" -ge 1 ] || die "At least 1 arguments required, $# provided"

version="${1}"
# append project path if provided

project_yaml=".chainloop.yml"
# manual override
if [[ -n "${2}" ]]; then
    project_yaml="${2}"
fi

## Changes in .chainloop.yml
sed -i "s#^projectVersion:.*#projectVersion: ${version}#g" "${project_yaml}"