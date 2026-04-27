FROM golang:1.26.2-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder
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
