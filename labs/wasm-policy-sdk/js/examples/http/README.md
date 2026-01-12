# HTTP Request Policy Example

A policy demonstrating HTTP requests with hostname restrictions using the Chainloop JavaScript SDK.

## What This Policy Does

This policy demonstrates how to:
1. Make external HTTP GET requests from within a policy
2. Parse JSON responses from APIs
3. Validate API response structure
4. Handle hostname restrictions for security

The example fetches data from httpbin.org and validates that the response contains a "slideshow" field.

## Security Features

**Hostname Restrictions**: The policy engine enforces hostname restrictions to prevent:
- Data exfiltration to unauthorized domains
- Malicious API calls
- Uncontrolled external dependencies

Only hostnames explicitly allowed via `--allowed-hostnames` flag can be accessed.

## Building

Install dependencies:

```bash
npm install
```

Build the policy:

```bash
npm run build
```

This will:
1. Use esbuild to bundle the policy with the Chainloop SDK
2. Use extism-js to compile the bundle to WASM

The build process creates:
- `dist/policy.js` - Bundled JavaScript (includes SDK)
- `policy.wasm` - Compiled WASM binary

## Testing

### Test with allowed hostname:

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material data.json \
  --kind EVIDENCE \
  --allowed-hostnames httpbin.org
```

Expected: **PASS** - Successfully fetches and validates slideshow data

### Test with blocked hostname:

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material data.json \
  --kind EVIDENCE \
  --allowed-hostnames example.com
```

Expected: **ERROR** - Request blocked with message:
```
HTTP request blocked - hostname 'httpbin.org' is not in the allowed hosts list
```

### Test with custom URL:

```bash
cat > /tmp/custom-url.json << 'EOF'
{
  "check_url": "https://api.github.com/zen"
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/custom-url.json \
  --kind EVIDENCE \
  --allowed-hostnames api.github.com
```

Expected: **FAIL** - Response doesn't have slideshow field (API validation fails)

### Test with multiple allowed hostnames:

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material data.json \
  --kind EVIDENCE \
  --allowed-hostnames httpbin.org,api.github.com,example.com
```

Expected: **PASS** - httpbin.org is in the allowed list

## Key Concepts Demonstrated

1. **HTTP GET requests**: Using `httpGetJSON()` to fetch external data
2. **Hostname restrictions**: Security enforcement at the policy engine level
3. **JSON response parsing**: Automatic parsing of API responses
4. **Error handling**: Catching and reporting HTTP failures
5. **Response validation**: Checking API response structure
6. **Material extraction**: Getting URL from policy material
7. **Conditional logic**: Validation based on API response content

## SDK Functions Used

- `getMaterialJSON()` - Parse material containing URL to check
- `httpGetJSON(url)` - Perform HTTP GET request and parse JSON response
- `success()` - Create success result
- `skip()` - Skip if material doesn't have required fields
- `fail()` - Create failure with error message
- `outputResult()` - Output validation result
- `result.addViolation()` - Add violation for failed validation
- `logInfo()` - Log request progress and results
- `logError()` - Log HTTP failures
- `run()` - Execution wrapper with error handling

## Understanding HTTP Restrictions

The policy engine enforces hostname restrictions at runtime:

```
Policy Material → Extract URL → Check Hostname → Allow/Block Request
```

**Allowed**: Request proceeds, response returned to policy
**Blocked**: Exception thrown with helpful error message

This prevents policies from:
- Calling arbitrary external services
- Leaking sensitive data to unauthorized endpoints
- Creating security vulnerabilities

## Material Format

The policy expects a JSON material with a `check_url` field:

```json
{
  "check_url": "https://httpbin.org/json"
}
```

## Files

- `policy.js` - Main policy implementation
- `policy.d.ts` - TypeScript type definitions for WASM exports
- `policy.yaml` - Policy configuration
- `data.json` - Sample material with URL to check
- `esbuild.js` - Build configuration
- `package.json` - npm package configuration
- `README.md` - This file

## Integration with Chainloop

Add to workflow contract:

```yaml
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: external-validation
spec:
  materials:
    - type: EVIDENCE
      name: api-check

  policies:
    materials:
      - ref: ./http/policy.yaml
        selector:
          name: api-check
        with:
          allowed_hostnames:
            - httpbin.org
            - api.trusted-service.com
```

## Real-World Use Cases

### 1. Vulnerability Database Checks

```javascript
const vulnResp = httpGetJSON(`https://api.osv.dev/v1/query?package=${packageName}`);
if (vulnResp.vulns && vulnResp.vulns.length > 0) {
  result.addViolation(`Found ${vulnResp.vulns.length} vulnerabilities`);
}
```

### 2. License Validation

```javascript
const licenseResp = httpGetJSON(`https://api.clearlydefined.io/definitions/${purl}`);
if (!approvedLicenses.includes(licenseResp.licensed.declared)) {
  result.addViolation(`License ${licenseResp.licensed.declared} not approved`);
}
```

### 3. Package Registry Verification

```javascript
const npmResp = httpGetJSON(`https://registry.npmjs.org/${packageName}`);
if (!npmResp.versions[version]) {
  result.addViolation(`Version ${version} not found in npm registry`);
}
```

### 4. Internal API Validation

```javascript
const approvalResp = httpGetJSON(`https://internal-api.company.com/approvals/${buildId}`);
if (approvalResp.status !== 'approved') {
  result.addViolation(`Build ${buildId} not approved by security team`);
}
```

## Error Handling

The policy handles different types of errors:

1. **Hostname blocked**: Clear error message with hint to add hostname
2. **Network failures**: Connection errors, timeouts
3. **HTTP errors**: 404, 500, etc.
4. **JSON parsing errors**: Invalid response format
5. **Missing fields**: Material doesn't have `check_url`

## Advanced Usage

### Custom Headers (when supported)

```javascript
// Future SDK enhancement
const resp = httpGetJSON(url, {
  headers: {
    'Authorization': 'Bearer token',
    'User-Agent': 'Chainloop-Policy/1.0'
  }
});
```

### POST Requests

```javascript
// Use httpPostJSON for POST requests
const resp = httpPostJSON(url, {
  query: 'data',
  filters: ['option1', 'option2']
});
```

## Security Best Practices

1. **Minimize external calls**: Only call APIs when necessary
2. **Use specific hostnames**: Don't allow wildcards or broad domains
3. **Validate responses**: Check response structure before using data
4. **Handle errors gracefully**: Don't expose sensitive error details
5. **Use HTTPS only**: Ensure all URLs use HTTPS protocol
6. **Timeout consideration**: Be aware of policy execution time limits

## Troubleshooting

**Error: "hostname not in allowed hosts list"**
→ Add the hostname using `--allowed-hostnames` flag

**Error: "Material missing 'check_url' field"**
→ Ensure your material JSON has a `check_url` field with a valid URL

**Error: "HTTP request failed with status 404"**
→ The URL endpoint doesn't exist or has moved

**Timeout errors**
→ The external service is slow or unavailable

## License

Apache License 2.0
