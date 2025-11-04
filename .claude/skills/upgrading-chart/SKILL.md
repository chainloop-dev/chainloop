---
name: upgrading-chart
description: Upgrades Helm chart dependencies (PostgreSQL, Vault) in the Chainloop project, including vendorized charts, container images, and CI/CD workflows. Use when the user mentions upgrading Helm charts, Bitnami dependencies, PostgreSQL chart, or Vault chart. CRITICAL - Major version upgrades are FORBIDDEN and must be escalated.
---

# Upgrading Helm Chart Dependencies

This skill automates the upgrade process for Helm chart dependencies in the Chainloop project. Supports PostgreSQL and Vault (both Bitnami charts).

## CRITICAL RESTRICTIONS

**Version Upgrade Rules**:
- Patch upgrades (1.2.3 → 1.2.4): ALLOWED
- Minor upgrades (1.2.x → 1.3.x): ALLOWED
- Major upgrades (1.x.x → 2.x.x): **FORBIDDEN - STOP IMMEDIATELY**

**MANDATORY**: If major version upgrade is detected, STOP the process and inform the user that manual review is required.

## Upgrade Types

The skill supports two upgrade types:

1. **Specific Image Upgrade**: Update container image to specific version (chart unchanged)
2. **Chart Minor Version Upgrade**: Update chart to latest minor version (may include image updates)

**IMPORTANT**: Container images are ONLY updated as part of chart upgrades, never independently (unless Type 1).

## Process

### 1. Identify Upgrade Type

Ask the user which type of upgrade they want:
- Type 1: Specific image version upgrade
- Type 2: Latest minor chart version upgrade

Also ask which chart: `postgresql` or `vault`

### 2. Pre-Upgrade Validation

Check current state:
```bash
cat deployment/chainloop/charts/<chart-name>/Chart.yaml | grep "^version:"
cat deployment/chainloop/charts/<chart-name>/Chart.yaml | grep "^appVersion:"
```

### 3. Version Compatibility Check

For any version change, validate that major version remains the same:
```bash
CURRENT_MAJOR=$(echo "$CURRENT_VERSION" | cut -d. -f1)
TARGET_MAJOR=$(echo "$TARGET_VERSION" | cut -d. -f1)

if [ "$CURRENT_MAJOR" != "$TARGET_MAJOR" ]; then
    echo "FORBIDDEN: Major version upgrade detected"
    exit 1
fi
```

If major version upgrade detected, STOP and escalate.

## Type 1: Specific Image Upgrade

See [image-upgrade-process.md](image-upgrade-process.md) for detailed steps.

**Summary**:
1. Locate target container image in [Bitnami Containers](https://github.com/bitnami/containers)
2. Find commit with release message pattern
3. Extract APP_VERSION from Dockerfile
4. Update `deployment/charts/<chart-name>/Chart.yaml` appVersion
5. Update `.github/workflows/build_external_container_images.yaml` commit hash

## Type 2: Chart Minor Version Upgrade

See [chart-upgrade-process.md](chart-upgrade-process.md) for detailed steps.

**Summary**:
1. Locate target chart version in [Bitnami Charts](https://github.com/bitnami/charts) CHANGELOG.md
2. Validate minor version upgrade only
3. Download and extract target chart
4. Check for image changes (compare Chart.yaml)
5. If images changed, update container image references
6. Vendorize chart update (copy files)
7. Update dependencies in correct order
8. Update main chart dependency version
9. Clean up temporary files

## Verification

After any upgrade type, run:
```bash
# Lint charts
helm lint deployment/charts/<chart-name>
helm lint deployment/chainloop

# Template validation
helm template deployment/charts/<chart-name>
helm template deployment/chainloop

# Local testing
cd devel && docker compose up

# Verify image consistency
grep -r "appVersion\|image.*tag" deployment/charts/<chart-name>/
```

## Files Modified

See [files-modified.md](files-modified.md) for complete list.

## Troubleshooting

Common issues:
- **Image Version Mismatch**: Verify APP_VERSION matches Chart.yaml appVersion
- **Build Failures**: Check commit reference in build workflow
- **Dependency Conflicts**: Verify dependencies updated in correct order (vendorized first, then main chart)

## Rollback

If issues occur:
```bash
git checkout HEAD -- deployment/
find deployment/ -name "Chart.lock" -delete
cd deployment/chainloop && helm dependency build
cd ../../devel && docker compose down && docker compose up
```

## Important Notes

- Dex is self-managed and follows a separate process (not covered by this skill)
- Always use commit hashes for reproducibility
- Dependencies must be updated in correct order: vendorized chart first, then main chart
- Container images are found in Bitnami Containers repo, charts in Bitnami Charts repo
