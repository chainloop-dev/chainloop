# Chart Dependencies Update Process

This document outlines the process for upgrading chart dependencies in the Chainloop project. The current dependencies are Vault and PostgreSQL, both based on Bitnami Charts, with vendorized versions stored in the repository.

## Overview

The Chainloop project uses vendorized Helm charts for its dependencies:
- **PostgreSQL**: Database backend (Bitnami chart)
- **Vault**: Secret storage (Bitnami chart)  
- **Dex**: OIDC provider (self-managed, not upgraded via this process)

## Step-by-Step Upgrade Process

### 1. Check Container Image Versions

1. Visit [Bitnami Containers](https://github.com/bitnami/containers)
2. Find the specific image (e.g., `bitnami/postgresql` or `bitnami/vault`)
3. Navigate the folder structure:
    - `2/debian-12` means:
        - `2`: major branch version
        - `debian-12`: distribution version
4. In the commit history, look for "Release ..." commits to find the container image version
5. Inside the `Dockerfile`, check the `APP_VERSION` environment variable
    - This is the application version used in the vendorized chart

### 2. Locate the Chart to Upgrade

1. Navigate to [Bitnami Charts](https://github.com/bitnami/charts)
2. Find the chart you want to upgrade (e.g., `postgresql` or `vault`)
3. **Note**: `dex` is self-managed and not upgraded through this process

### 3. Identify the Chart Version

1. Navigate to the `bitnami/<chart-name>` folder in the Bitnami Charts repository
2. Review the Git history and look for commits containing "Release"
3. These commits indicate chart version updates and contain version information but it does not say which container image version is used for that it needs to be checked individually

### 4. Check Container Images Used by the Chart

1. Inside the chart's `Chart.yaml`, look under the `images` section
2. This lists all container images bundled with that chart version
3. Chart upgrades are typically triggered by new container images (e.g., security fixes)

### 5. Match Image to Chart Version

1. Locate the Bitnami chart version that includes the container image you want
2. Ensure the `APP_VERSION` matches your requirements
3. Verify if other dependencies/images also need updating
4. Check for any breaking changes or migration requirements

### 6. Vendorize the Chart Update

1. **Package the chart**:
   ```bash
   helm package <chart-name> --version <target-version>
   ```

2. **Extract to deployment directory**:
   ```bash
   # Decompress the chart under deployment/charts/
   tar -xzf <chart-name>-<version>.tgz -C deployment/charts/
   ```

3. **Update image tags in deployment configuration**:
   - Edit `deployment/chainloop/Chart.yaml`
   - Update image tags so local rebuilds reference the correct images
   - The image tag should match the `APP_VERSION` from step 4

### 7. Update External Image Build References

1. Edit `.github/build_external_container_image.md`
2. Add the commit reference found in the Bitnami Containers repository
3. Update the appropriate section with the new commit hash and version information

## Verification Steps

After completing the upgrade process:

1. **Test locally**:
   ```bash
   cd devel && docker compose up
   ```

2. **Verify chart deployment**:
   ```bash
   helm lint deployment/charts/<chart-name>
   helm template deployment/charts/<chart-name>
   ```

3. **Check container image references**:
   - Ensure all image tags are correctly updated
   - Verify SHA256 digests if used

4. **Test integration**:
   - Run integration tests
   - Verify services start correctly
   - Test basic functionality

## Important Notes

1. **Vendorized Charts**: The repository contains local copies of charts for consistency and security
2. **Image Security**: Always verify container image versions for security updates
3. **Breaking Changes**: Review chart changelogs for breaking changes before upgrading
4. **Dex Exception**: Dex OIDC provider is self-managed and uses a different upgrade process
5. **Rollback Plan**: Keep previous chart versions available for quick rollback if needed

## Files Modified in This Process

### Chart Files
- `deployment/charts/<chart-name>/` - Vendorized chart directory
- `deployment/chainloop/Chart.yaml` - Image tag references

### CI/CD Configuration  
- `.github/build_external_container_image.md` - External image build references

### Testing
- Local Docker Compose configurations may need updates for new versions

## Troubleshooting

Common issues and solutions:

1. **Image Pull Failures**: Verify image tags and availability in registries
2. **Configuration Changes**: Check for deprecated or changed configuration options
3. **Dependency Conflicts**: Ensure all chart dependencies are compatible
4. **Migration Requirements**: Some upgrades may require data migration steps
5. **No Version of Container Image found in Bitnami Containers**: If you cannot find the required version, check the history of the specified folder (e.g., `bitnami/postgresql/16/debian-12`) to see if it was removed or renamed.
