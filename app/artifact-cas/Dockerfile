FROM golang:1.24.4@sha256:be70d93633d07a2acae4ff3401672b04f23e5850b0248d65c23e30dc75dded09 AS builder

# Not linked libraries since it will be injected into a scratch container
ENV CGO_ENABLED=0

COPY . /src
WORKDIR /src/app/artifact-cas

RUN make build

FROM scratch

COPY --from=builder /src/app/artifact-cas/bin /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/


WORKDIR /app
ENTRYPOINT [ "./artifact-cas", "--conf", "/data/conf"]
