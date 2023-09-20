FROM golang:1.20@sha256:6e1a67ec11cc4eaa7a9699bc6e5385b00001669531284005ebd2ada4129138d9 AS builder

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
