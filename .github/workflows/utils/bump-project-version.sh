#!/usr/bin/env bash

# Update the Chainloop project version in .chainloop.yml to the provided version + "+next"

set -e

die () {
   echo >&2 "$@"
   echo "usage: bump-project-version.sh <version> [configFile]"
   exit 1
}

## debug if desired
if [[ -n "${DEBUG}" ]]; then
   set -x
fi

[ "$#" -ge 1 ] || die "Version argument is required"

version="${1}"
project_yaml=".chainloop.yml"
# manual override
if [[ -n "${2}" ]]; then
   project_yaml="${2}"
fi

# Append "+next" to the version
version_with_next="${version}+next"

# Update the project yaml file
sed -i "s#^projectVersion:.*#projectVersion: ${version_with_next}#g" "${project_yaml}"

