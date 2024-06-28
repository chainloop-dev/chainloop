---
title: Signing and verification methods
---

All attestations are bundled in a DSSE Envelope and signed before being sent to Chainloop Control Plane. This happens transparently while performing a `chainloop attestation push` command.

Verification of the attestation integrity is done through the `workflow run describe --verify true` command.

The signing and verification methods used by Chainloop CLI will depend on the different options provided.

These are the various signing and verification methods currently supported by Chainloop:

:::note
Some of these signing methods are inherited from the amazing Sigstore community products. Please make sure to check 
[their documentation](https://docs.sigstore.dev/signing/signing_with_blobs/#signing-with-a-key) on the usage of the `--key` argument for key references.
:::

| Method                                                     | Signing (`chainloop att push`)                                                                                                                                                                                                                     | Verifying (`chainloop wf run describe --verify true`)                                                                                                       |
|------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------| 
| Cosign key                                                 | `--key cosign.key`                                                                                                                                                                                                                                 | `--key cosign.pub`                                                                                                                                          |
| KMS                                                        | `--key awskms://<KeyID>`                                                                                                                                                                                                                           | `--key awskms://<KeyID>`                                                                                                                                    | 
| PKCS#11                                                    | `--key pkcs11://<KeyId>`                                                                                                                                                                                                                           | `--key pkcs11://<KeyId>`                                                                                                                                    |
| Kubernetes secret                                          | `--key k8s://<namespace>/<secretName>` (where `cosign.key` and `cosign.password` secrets are expected)                                                                                                                                             | `--key k8s://<namespace>/<secretName>` (where `cosign.pub` is expected)                                                                                     |
| Gitlab secret                                              | `--key gitlab://<project>` (it will look for `COSIGN_PRIVATE_KEY`, `COSIGN_PASSWORD` variables)                                                                                                                                                    | `--key gitlab://<project>` (it will look for `COSIGN_PUBLIC_KEY` variable)                                                                                  |
| Ephemeral (file based CA)                                  | Configure your CA in [your deployment](https://github.com/chainloop-dev/chainloop/blob/main/deployment/chainloop/templates/controlplane/file_ca.secret.yaml) and omit the `--key` when pushing your attestation.                                   | Use `--cert cert.pem --cert-chain my-root.pem` in the `describe` command                                                                                    |
| Ephemeral ([EJBCA](https://github.com/Keyfactor/ejbca-ce)) | Connect your EJBCA instance to your Chainloop deployment using [these settings](https://github.com/chainloop-dev/chainloop/blob/main/deployment/chainloop/templates/controlplane/ejbca_ca.secret.yaml). Omit `--key` when pushing the attestation. | Use your verification material with `--cert` and `--cert-chain` options in the `wf run describe` command. <br/>See [bundles](#bundles)                      |
| [SignServer](https://www.signserver.org/)                  | You can sign with your instance of SignServer with `--key signserver://host/worker`                                                                                                                                                                | Both signing certificate and chain must be provided out of band. Use `--cert signingcert.pem --cert-chain root.pem` to verify. <br/>See [bundles](#bundles) | 

#### Bundles 
When signing with a verification method that supports it (like ephemeral certificates), you can download the verification material, to be used later during the verification process.

Just add `--bundle my-bundle.json` to the `push` command. Then, you can use the material to verify the attestation:
```
> cat my-bundle.json | jq -r ".verificationMaterial.x509CertificateChain.certificates.[].rawBytes" | base64 --decode | openssl x509 -inform DER -outform PEM -out cert.pem
> chainloop wf run describe --digest ... --verify true --cert cert.pem --cert-chain my-root.pem
```

#### Not yet supported

The following methods are work in progress and **not yet supported**.

| Method                                                                         | Signing                                                   | Verifying                                                                               |
|--------------------------------------------------------------------------------|-----------------------------------------------------------|-----------------------------------------------------------------------------------------|
| Ephemeral (file based CA) with verification bundle stored in the control plane | No key needed                                             | No verification material needed (will be automatically downloaded from Chainloop Vault) | 
| x509 certificate                                                               | `--key privatekey --cert cert.pem --cert-chain chain.pem` | `--cert cert.pem --cert-chain chain.pem`                                                | 
