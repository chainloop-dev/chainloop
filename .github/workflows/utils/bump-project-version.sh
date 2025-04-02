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

# Load the previous version and bump appropriately
version=$(cat "${project_yaml}" | awk '/^projectVersion:/ {
    version = $2;
    if (version ~ /-rc/) {
        # Handle release candidate versions (e.g., v1.0.0-rc.1 -> v1.0.0-rc.2)
        split(version, parts, /-rc\./);
        rc_num = parts[2] + 1;
        print parts[1] "-rc." rc_num;
    }
    else {
        # Load the previous version and BUMP THE MINOR
        # Handle minor version bumps (e.g., v1.0.0 -> v1.1.0)
        split(version, ver_parts, /\./);
        ver_parts[2] = ver_parts[2] + 1;
        print ver_parts[1] "." ver_parts[2] "." ver_parts[3];
    }
}')

# Update the project yaml file
sed -i "s#^projectVersion:.*#projectVersion: ${version}#g" "${project_yaml}"

