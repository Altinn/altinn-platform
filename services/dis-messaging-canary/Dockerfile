# Build the dis-messaging-canary binary
FROM golang:1.27.4@sha256:5f1b98fa02fe396ebce2a0227a0e08309e11792915b9dd2e1f71053d7e7e6666 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go module files first. The download layer then stays in the cache
# when only the source changes.
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# Copy the Go source
COPY cmd/ cmd/
COPY internal/ internal/

# GOARCH has no default value, so the binary gets the platform of the build.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o dis-messaging-canary ./cmd

# The distroless static image has CA certificates and a non-root user, and no
# shell. The canary writes no files, so the root file system can be read-only.
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/dis-messaging-canary .
USER 65532:65532

ENTRYPOINT ["/dis-messaging-canary"]
