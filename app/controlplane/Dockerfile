FROM golang:1.22@sha256:c2010b9c2342431a24a2e64e33d9eb2e484af49e72c820e200d332d214d5e61f AS builder

# Not linked libraries since it will be injected into a scratch container
ENV CGO_ENABLED=0

COPY . /src
WORKDIR /src/app/controlplane

RUN make build

FROM scratch

COPY --from=builder /src/app/controlplane/bin /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/


WORKDIR /app
ENTRYPOINT [ "./control-plane", "--conf", "/data/conf"]
