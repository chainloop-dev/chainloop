FROM golang:1.21@sha256:afccce40fb4a6b6a80d0386d6296737c68207f8d69086d0e16aa9cb9dbb753db AS builder

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
