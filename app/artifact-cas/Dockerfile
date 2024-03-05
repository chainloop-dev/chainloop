FROM golang:1.22.1@sha256:46a86b411554728154e56f9719426a47e5ded3cab7adb1ecb22a988f486e99ae AS builder

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
