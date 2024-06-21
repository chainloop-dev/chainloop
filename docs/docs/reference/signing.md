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

<table>
<thead>
<tr><td>Method</td><td>Signing (`chainloop att push`)</td><td>Verifying (`chainloop wf run describe --verify true`)</td></tr>
</thead>
<tbody>
<tr><td>Cosign key</td><td>`--key cosign.key`</td><td>`--key cosign.pub`</td></tr>
<tr><td>KMS</td><td>`--key awskms://<KeyID>`</td><td>`--key awskms://<KeyID>`</td></tr>
<tr><td>PKCS#11</td><td>`--key pkcs11://<KeyId>`</td><td>`--key pkcs11://<KeyId>`</td></tr>
<tr>
    <td>Kubernetes secret</td><td>`--key k8s://<namespace>/<secretName>` (where `cosign.key` and `cosign.password` secrets are expected)</td>
    <td>`--key k8s://<namespace>/<secretName>` (where `cosign.pub` is expected)</td>
</tr>
<tr>
    <td>Gitlab secret</td><td>`--key gitlab://<project>` (it will look for `COSIGN_PRIVATE_KEY`, `COSIGN_PASSWORD` variables)</td>
    <td>`--key gitlab://<project>` (it will look for `COSIGN_PUBLIC_KEY` variable)</td>
</tr>
<tr>
    <td>Ephemeral (file based CA)</td>
    <td>Configure your CA in [your deployment](https://github.com/chainloop-dev/chainloop/blob/main/deployment/chainloop/templates/controlplane/file_ca.secret.yaml) and omit the `--key` when pushing your attestation.
        To download the verification material for later verification, add `--bundle my-bundle.json` to the push options</td>
    <td>If verification material was downloaded while signing, you can use it to verify:`cat my-bundle.json | jq -r ".verificationMaterial.x509CertificateChain.certificates.[].rawBytes" | base64 --decode | openssl x509 -inform DER -outform PEM -out cert.pem`. Then you can use `--cert cert.pem --cert-chain my-root.pem` in the `describe` command</td>
</tr>
<tr><td>[SignServer](https://www.signserver.org/)</td><td>You can sign with your instance of SignServer with `--key signserver://host/worker`</td><td>Both signing certificate and chain must be provided out of band. Use `--cert signingcert.pem --cert-chain root.pem` to verify</td></tr>
</tbody>
</table>

The following methods are work in progress and **not yet supported**.

<table>
<thead>
<tr><td>Method</td><td>Signing</td><td>Verifying</td></tr>
</thead>
<tbody>
<tr><td>Ephemeral (file based CA) with verification bundle stored in the control plane</td><td>No key needed</td><td>No verification material needed (will be automatically downloaded from Chainloop Vault</td></tr>
<tr><td>x509 certificate</td><td>`--key privatekey --cert cert.pem --cert-chain chain.pem`</td><td>`--cert cert.pem --cert-chain chain.pem`</td></tr>

</tbody>
</table>
