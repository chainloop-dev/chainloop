# Artifact Content Addressable Storage (CAS) Client code

Client code used to talk to the [Artifact Storage Proxy](/app/artifact-cas/).

It's a [bytestream gRPC client](https://pkg.go.dev/google.golang.org/api/transport/bytestream) that currently supports download by content digest (sha256) and upload methods.
