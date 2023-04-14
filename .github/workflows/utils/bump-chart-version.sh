#!/usr/bin/env bash

# Bump Helm Chart version, and appVersion to a given version number

set -e

die () {
    echo >&2 "$@"
    echo "usage: bump.sh [chartYamlFile] [version]"
    exit 1
}

## debug if desired
if [[ -n "${DEBUG}" ]]; then
    set -x
fi

[ "$#" -eq 2 ] || die "2 arguments required, $# provided"

chart_yaml="${1}"
version="${2}"


sed -i "s#^appVersion:.*#appVersion: ${version}#g" "${chart_yaml}"
sed -i "s#^version:.*#version: ${version}#g" "${chart_yaml}"
