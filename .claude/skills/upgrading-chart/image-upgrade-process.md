# Specific Image Upgrade Process (Type 1)

This process upgrades a container image to a specific version without changing the chart version.

## Step 1: Locate Target Container Image

1. Navigate to [Bitnami Containers](https://github.com/bitnami/containers)
2. Find image folder: `bitnami/<image-name>`
3. Check commit history: `https://github.com/bitnami/containers/commits/main/bitnami/<image-name>`
4. Find commit with message pattern: `Release <image>-<version>-<distro>-<distro-version>-r<revision>`
   - Example: `Release postgresql-15.3.0-debian-12-r1`
5. Note the commit hash
6. Open the Dockerfile in that commit
7. Extract the `APP_VERSION` environment variable value

**Example Dockerfile location**:
```
bitnami/containers/<image-name>/<major-version>/<distro>-<distro-version>/Dockerfile
```

## Step 2: Update Chart appVersion

Edit `deployment/charts/<chart-name>/Chart.yaml`:

```yaml
# Update only the appVersion field to match APP_VERSION from Dockerfile
appVersion: "X.X.X"

# Keep chart version unchanged
version: "Y.Y.Y"
```

## Step 3: Update Build Configuration

Edit `.github/workflows/build_external_container_images.yaml`:

Update the commit hash reference for the specific image to point to the commit identified in Step 1.

## Step 4: Verify Changes

```bash
# Check consistency
grep "appVersion" deployment/charts/<chart-name>/Chart.yaml
grep "<chart-name>" .github/workflows/build_external_container_images.yaml

# Lint and test
helm lint deployment/charts/<chart-name>
cd devel && docker compose up
```

## Files Modified

- `deployment/charts/<chart-name>/Chart.yaml` - appVersion only
- `.github/workflows/build_external_container_images.yaml` - commit hash
