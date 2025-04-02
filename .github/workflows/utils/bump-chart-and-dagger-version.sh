#!/usr/bin/env bash

# Bump Helm Chart version, appVersion to a given version number

set -e

die () {
    echo >&2 "$@"
    echo "usage: bump.sh [chartPath] [daggerPath] [version] [isCanary]"
    exit 1
}

## debug if desired
if [[ -n "${DEBUG}" ]]; then
    set -x
fi

[ "$#" -ge 3 ] || die "At least 3 arguments required, $# provided"

chart_yaml="${1}/Chart.yaml"
values_yaml="${1}/values.yaml"
dagger_main="${2}/main.go"
semVer="${3}"

## Changes in Chart.yaml
# If we are bumping to a canary version, we want to 
# - Replace the patch segment in the version (build or pre-release component are not valid)
# - Append `canary` to the chart Name
isCanary="${3:-false}"
if [[ "${isCanary}" == "true" ]]; then
    # i.e 1.0.2 => 1.0.2024122233
    chartVersion=$(cat ${chart_yaml} | awk -F'[ .]' '/^version:/ {print $2"."$3}').${semVer}

    sed -i "s#^version:.*#version: ${chartVersion}#g" "${chart_yaml}"
    sed -i 's/^name: \(.*\)/name: \1-canary/' "${chart_yaml}"
else
    # We are not bumping to a canary version so we want to
    # Bump chart version MINOR and reset PATCH segment
    # A new release means a bump in the minor segment of the Chart and a reset of the patch one
    # i.e 1.0.2 => 1.1.0
    chartVersion=$(cat ${chart_yaml} | awk -F'[ .]' '/^version:/ {print $2"."$3+1"."0}')
    sed -i "s#^version:.*#version: ${chartVersion}#g" "${chart_yaml}"
fi

# AppVersion represents the container version
sed -i "s#^appVersion:.*#appVersion: ${semVer}#g" "${chart_yaml}"
# We want to also replace the images annotation tags
sed -i "s/:v.*/:${semVer}/g" "${chart_yaml}"

## Changes images in Values.yaml
sed -i "s/tag: .*/tag: \"${semVer}\"/g" "${values_yaml}"

## Update Dagger version
sed -i "s/chainloopVersion = v.*\"/chainloopVersion = \"${semVer}\"/" "${dagger_main}"

