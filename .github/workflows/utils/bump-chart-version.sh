#!/usr/bin/env bash

# Bump Helm Chart version, appVersion to a given version number

set -e

die () {
    echo >&2 "$@"
    echo "usage: bump.sh [chartPath] [version]"
    exit 1
}

## debug if desired
if [[ -n "${DEBUG}" ]]; then
    set -x
fi

[ "$#" -eq 2 ] || die "2 arguments required, $# provided"

chart_yaml="${1}/Chart.yaml"

# Remove v prefix if provided
semVer="$(echo ${2} | sed -e 's/^v\(.*\)/\1/')"


# AppVersion includes a v prefix
sed -i "s#^appVersion:.*#appVersion: v${semVer}#g" "${chart_yaml}"

# Bump chart version patch segment
chartVersion=$(cat ${chart_yaml} | awk -F'[ .]' '/^version:/ {print $2"."$3"."$4+1}')
sed -i "s#^version:.*#version: ${chartVersion}#g" "${chart_yaml}"
