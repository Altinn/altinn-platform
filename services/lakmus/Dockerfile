FROM golang:1.26.1-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
COPY test/ test/

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o lakmus cmd/main.go

FROM gcr.io/distroless/static:nonroot@sha256:0376b514983f02c630de9ed8abadd33968ddb778f9f383412a12babe639cbaaa

WORKDIR /
COPY --from=builder /workspace/lakmus .

USER 65532:65532

ENTRYPOINT ["/lakmus"]
