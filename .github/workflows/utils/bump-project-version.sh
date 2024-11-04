#!/usr/bin/env bash

# Bump the Chainloop project version to the next minor version to the version defined .chainloop.yml

set -e

die () {
   echo >&2 "$@"
   echo "usage: bump-project-version.sh [configFile]"
   exit 1
}

## debug if desired
if [[ -n "${DEBUG}" ]]; then
   set -x
fi

project_yaml=".chainloop.yml"
# manual override
if [[ -n "${1}" ]]; then
   project_yaml="${1}"
fi

# load the previous version and BUMP THE MINOR
version=$(cat ${project_yaml} | awk -F'[ .]' '/^projectVersion:/ {print $2"."$3+1"."0}')

## Changes in .chainloop.yml
sed -i "s#^projectVersion:.*#projectVersion: ${version}#g" "${project_yaml}"
