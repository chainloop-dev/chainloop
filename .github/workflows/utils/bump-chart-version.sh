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

# Bump chart version MINOR and reset PATCH segment
# A new release means a bump in the minor segment of the Chart and a reset of the patch one
# i.e 1.0.2 => 1.1.0
chartVersion=$(cat ${chart_yaml} | awk -F'[ .]' '/^version:/ {print $2"."$3+1"."0}')
sed -i "s#^version:.*#version: ${chartVersion}#g" "${chart_yaml}"
