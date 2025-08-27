# Chainloop Chart Dependencies Upgrade Runbook

## Overview

This runbook provides step-by-step instructions for upgrading Helm chart dependencies in the Chainloop project. Follow this process to ensure safe, consistent upgrades while maintaining system stability.

### Dependencies Managed
- **PostgreSQL**: Database backend (Bitnami chart)
- **Vault**: Secret storage (Bitnami chart)
- **Dex**: OIDC provider *(self-managed, separate process)*

---

## CRITICAL RESTRICTIONS

### Version Upgrade Rules
| Upgrade Type | Example | Status |
|--------------|---------|--------|
| Patch upgrade | `1.2.3` → `1.2.4` | ALLOWED |
| Minor upgrade | `1.2.x` → `1.3.x` | ALLOWED |
| Major upgrade | `1.x.x` → `2.x.x` | **FORBIDDEN - STOP IMMEDIATELY** |

**MANDATORY CHECK**: Any major version upgrade attempt must **STOP** the process and escalate for manual review.

---

## Upgrade Types

### Chart Version Upgrade Only
- **Purpose**: Upgrade chart to latest minor version
- **Scope**: Update chart version and potentially images
- **Use Case**: Feature updates, security patches, regular maintenance
- **Rule**: Container images are **ONLY** updated as part of chart upgrades, never independently

---

## Pre-Upgrade Validation

### Step 1: Identify Current State
```bash
# Check current chart version
cat deployment/chainloop/charts/<chart-name>/Chart.yaml | grep "^version:"

# Check current app version
cat deployment/chainloop/charts/<chart-name>/Chart.yaml | grep "^appVersion:"
```

### Step 2: Version Compatibility Check
```bash
# Example validation
CURRENT_MAJOR=$(echo "$CURRENT_VERSION" | cut -d. -f1)
TARGET_MAJOR=$(echo "$TARGET_VERSION" | cut -d. -f1)

if [ "$CURRENT_MAJOR" != "$TARGET_MAJOR" ]; then
    echo "FORBIDDEN: Major version upgrade detected"
    echo "Current: $CURRENT_VERSION → Target: $TARGET_VERSION"
    exit 1
fi
```

### Step 3: Stop Conditions
**STOP IMMEDIATELY if**:
- Major version change detected
- Breaking changes require manual intervention
- Dependencies conflict

---

## Reference Resources

### Bitnami Charts Repository Structure
```
bitnami/charts/
└── <chart-name>/
    ├── Chart.yaml          # Chart metadata and version
    ├── CHANGELOG.md        # Version history and changes
    ├── values.yaml         # Default configuration
    └── templates/          # Helm templates
```

### Bitnami Containers Repository Structure
```
bitnami/containers/
└── <image-name>/
    └── <major-version>/
        └── <distro>-<distro-version>/
            ├── Dockerfile      # Contains APP_VERSION
            └── ...
```

### Finding Resources
| Resource | Location | Purpose |
|----------|----------|---------|
| Chart versions | [Bitnami Charts](https://github.com/bitnami/charts) + `CHANGELOG.md` | Find available chart versions |
| Container images | [Bitnami Containers](https://github.com/bitnami/containers) + commit history | Find image versions and commit hashes |
| Release tags | Commit messages: `Release <name>-<version>-<distro>-<distro-version>-r<revision>` | Identify specific releases |

---

## Type 1: Specific Image Upgrade Process

### Step 1: Locate Target Container Image
1. Navigate to [Bitnami Containers](https://github.com/bitnami/containers)
2. Find image folder: `bitnami/<image-name>`
3. Check commit history: `https://github.com/bitnami/containers/commits/main/bitnami/<image-name>`
4. Find commit with message: `Release <image>-<version>-<distro>-<distro-version>-r<revision>`
5. Note the commit hash and APP_VERSION from Dockerfile

### Step 2: Update Image References
```bash
# Edit Chart.yaml - update appVersion only
vi deployment/charts/<chart-name>/Chart.yaml

# Update appVersion field to match container's APP_VERSION
# Keep chart version unchanged
```

### Step 3: Update Build Configuration
```bash
# Edit build workflow to reference the correct commit
vi .github/workflows/build_external_container_images.yaml

# Update commit hash for the specific image
```

---

## Type 2: Latest Minor Version Chart Upgrade Process

### Step 1: Locate Target Chart Version
1. Navigate to [Bitnami Charts](https://github.com/bitnami/charts)
2. Open `bitnami/<chart-name>/CHANGELOG.md`
3. Find latest minor version (ensure no major version change)
4. Note target chart version

### Step 2: Version Validation
```bash
# MANDATORY: Verify minor upgrade only
CURRENT_CHART_VERSION="<current>"
TARGET_CHART_VERSION="<target>"

# Validate major version compatibility
# If major version differs, STOP PROCESS
```

### Step 3: Download and Extract Target Chart
```bash
# Pull chart to temporary location
helm pull bitnami/<chart-name> --version <target-version> --untar --untardir /tmp

# Examine the new chart structure
ls -la /tmp/<chart-name>/
```

### Step 4: Check for Image Changes
```bash
# Compare current vs target chart images
diff deployment/charts/<chart-name>/Chart.yaml /tmp/<chart-name>/Chart.yaml

# Look for changes in:
# - appVersion field
# - images section (if present)
# - dependencies
```

### Step 5: Update Container Images (if changed)
**Execute only if images changed in target chart**:

1. **Locate new image versions**:
   ```bash
   # For each changed image, find in Bitnami Containers
   # Pattern: <app-version>-<distro>-<distro-version>-r<revision>
   # Example: 15.3.0-debian-12-r1
   ```

2. **Get APP_VERSION from Dockerfile**:
   ```bash
   # Navigate to bitnami/containers/<image>/<major-version>/<distro>-<version>/
   # Extract APP_VERSION from Dockerfile
   ```

3. **Update build configuration**:
   ```bash
   # Update commit hash in build workflow
   vi .github/workflows/build_external_container_images.yaml
   ```

### Step 6: Vendorize Chart Update
```bash
# Replace vendorized chart with new version
cp -r /tmp/<chart-name>/* deployment/charts/<chart-name>/

# Update Chart.yaml if images changed
vi deployment/charts/<chart-name>/Chart.yaml
# Set appVersion to APP_VERSION from Bitnami Containers

# Update values.yaml if needed
# Replace docker.io/bitnami/* with Chainloop registry paths
vi deployment/charts/<chart-name>/values.yaml
```

### Step 7: Update Dependencies
```bash
# CRITICAL: Update dependencies in correct order

# 1. Update vendorized chart dependencies first
cd deployment/charts/<chart-name>
helm dependency update
helm dependency build

# 2. Update main chart dependency version
cd ../../chainloop
vi Chart.yaml  # Update dependency version to match vendorized chart

# 3. Update main chart dependencies
helm dependency update  
helm dependency build

cd ../..
```

### Step 8: Clean Up
```bash
# Remove temporary files
rm -rf /tmp/<chart-name>

# Verify working directory is clean
git status
```

---

## Verification & Testing

### Local Verification
```bash
# 1. Lint charts
helm lint deployment/charts/<chart-name>
helm lint deployment/chainloop

# 2. Template validation
helm template deployment/charts/<chart-name>
helm template deployment/chainloop

# 3. Local testing
cd devel && docker compose up

# 4. Integration testing
# Run your specific integration test suite
```

### Image Consistency Checks
```bash
# Verify consistency across:
# - Chart.yaml appVersion
# - values.yaml image tags  
# - Build workflow commit references

grep -r "appVersion\|image.*tag" deployment/charts/<chart-name>/
grep -r "<chart-name>" .github/workflows/build_external_container_images.yaml
```

---

## Files Modified

### Chart Files
- `deployment/charts/<chart-name>/Chart.yaml` - Chart version, appVersion
- `deployment/charts/<chart-name>/values.yaml` - Image references
- `deployment/charts/<chart-name>/Chart.lock` - Dependency lock
- `deployment/chainloop/Chart.yaml` - Main chart dependencies
- `deployment/chainloop/Chart.lock` - Main dependency lock

### CI/CD Configuration
- `.github/workflows/build_external_container_images.yaml` - Image build references

---

## Troubleshooting

| Issue | Symptoms | Solution |
|-------|----------|----------|
| **Image Version Mismatch** | Services fail to start | Verify APP_VERSION matches Chart.yaml appVersion |
| **Build Failures** | CI/CD fails to build images | Check commit reference contains required image versions |
| **Image Pull Failures** | Deployment fails | Ensure all image tags are consistent and updated |
| **Dependency Conflicts** | Helm dependency errors | Check compatibility between chart versions |
| **Missing Container Images** | Image not found errors | Check Bitnami Containers history for renamed/removed images |

### Debug Commands
```bash
# Check image references
grep -r "image:" deployment/charts/<chart-name>/

# Verify build configuration  
grep -A5 -B5 "<chart-name>" .github/workflows/build_external_container_images.yaml

# Check dependency status
helm dependency list deployment/chainloop

# Validate chart syntax
helm lint deployment/charts/<chart-name> --strict
```

---

## Emergency Procedures

### Rollback Steps
1. **Immediate**: Revert git changes
   ```bash
   git checkout HEAD -- deployment/
   ```

2. **Clean state**: Remove lock files and rebuild
   ```bash
   find deployment/ -name "Chart.lock" -delete
   cd deployment/chainloop && helm dependency build
   ```

3. **Verify**: Test rolled-back version
   ```bash
   cd devel && docker compose down && docker compose up
   ```

### Escalation Criteria
**Contact team lead when**:
- Major version upgrade is required
- Breaking changes affect core functionality
- Multiple dependency conflicts arise
- Data migration is required

---

## Process Checklist

### Pre-Upgrade
- [ ] Version compatibility verified (no major version change)
- [ ] Current state documented
- [ ] Backup/rollback plan confirmed

### During Upgrade
- [ ] Target version located and validated
- [ ] Image changes identified and updated
- [ ] Charts vendorized correctly
- [ ] Dependencies updated in correct order
- [ ] Build configuration updated (if needed)

### Post-Upgrade
- [ ] Local testing passed
- [ ] Chart linting clean
- [ ] Image consistency verified
- [ ] Integration tests passed
- [ ] Documentation updated
- [ ] Temporary files cleaned up
