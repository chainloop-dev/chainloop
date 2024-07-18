---
title: Policies
---

Starting with Chainloop [0.93.8](https://github.com/chainloop-dev/chainloop/releases/tag/v0.93.8), operators can attach policies to contracts. 
These policies will be evaluated against all attestation metadata and materials in any workflow with that contract.

### Policy specification
A policy can be defined in a YAML document, like this:
```yaml
# sbom-licenses.yaml
apiVersion: workflowcontract.chainloop.dev/v1
kind: Policy
metadata:
  name: sbom-licenses # (1)
spec:
  type: SBOM_CYCLONEDX_JSON # (2)
  embedded: | # (3)
    package main

    deny[msg] {
      count(without_license) > 0
      msg := "SBOM has components without licenses"
    }

    without_license = {comp.purl |
      some i
      comp := input.components[i]
      not comp.licenses
    }
```
In this particular example, we see:
* (1) policies have a name
* (2) they can be optionally applied to a specific type of material (check [the documentation](./operator/contract#material-schema) for the supported types). If no type is specified, a material name will need to be provided explicitly in the contract.
* (3) they have a policy script that it's evaluated against the material (in this case a CycloneDX SBOM report). Currently, only [Rego](https://www.openpolicyagent.org/docs/latest/policy-language/#learning-rego) policies are supported.

Policy scripts could also be specified in a detached form:
```yaml
...
spec:
  type: SBOM_CYCLONEDX_JSON
  path: my-script.rego
```

### Applying policies to contracts
When defining a contract, a new `policies` section can be specified. Policies can be applied to any material, but also to the attestation statement as a whole.
```yaml
schemaVersion: v1
materials:
  - name: sbom
    type: SBOM_CYCLONEDX_JSON
  - name: another-sbom
    type: SBOM_CYCLONEDX_JSON
policies:
  materials: # policies applied to materials
    - ref: sbom-licenses.yaml # (1)
  attestation: # policies applied to the whole attestation
    - ref: chainloop-commit.yaml # (2)
```
Here we can see that:
- (1) materials will be validated against `sbom-licenses.yaml` policy. But, since that policy has a `type` property set to `SBOM_CYCLONEDX_JSON`, all materials of that type (two in this case) will be evaluated. 
  
  If we wanted to only evaluate the policy against the `sbom` material, and skip any other, we should filter them by name:
  ```yaml
  policies:
    materials:
      - ref: sbom-licenses.yaml
        selector: # (3)
          name: sbom
  ```
  Here, we are making explicit that only `sbom` material must be evaluated by the `sbom-licenses.yaml` policy.
- (2) the attestation in-toto statement as a whole will be evaluated against `chainloo-commit.yaml`, which has a `type` property set to `ATTESTATION`. This brings the opportunity to validate global attestation properties, checking the presence of a material, etc. You can see this policy and other examples in the [examples folder](https://github.com/chainloop-dev/chainloop/tree/main/docs/examples/policies).

Finally, note that material policies are evaluated during `chainloop attestation add` commands, while attestation policies are evaluated in `chainloop attestation push` command.

### Rego scripts
Currently, policy scripts are assumed to be written in [Rego language](https://www.openpolicyagent.org/docs/latest/policy-language/#learning-rego). Other policy engines might be implemented in the future.
The only requirement of the policy is the existence of one or multiple `deny` rules, which evaluate to a **list of violation strings**.
For example, this policy script:
```yaml
    package main
    
    deny[msg] {
      not is_approved
      
      msg:= "Container image is not approved"
    }
    
    is_approved {
      input.predicate.materials[_].annotations["chainloop.material.type"] == "CONTAINER_IMAGE"
      
      input.predicate.annotations.approval == "true"
    }
```
when evaluated against an attestation, will generate the following output if the expected annotation is not present:
```json
{
    "deny": [
        "Container image is not approved"
    ]
}
```
Make sure you test your policies in https://play.openpolicyagent.org/, since you might get different results when using Rego V1 syntax, as there are [some breaking changes](https://www.openpolicyagent.org/docs/latest/opa-1/).