FROM golang:1.25.0@sha256:9e56f0d0f043a68bb8c47c819e47dc29f6e8f5129b8885bed9d43f058f7f3ed6 AS builder

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
