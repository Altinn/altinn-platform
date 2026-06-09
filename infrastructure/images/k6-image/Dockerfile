FROM golang:1.26-alpine@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f AS builder
RUN apk update && apk upgrade && apk add git

RUN go install go.k6.io/xk6/cmd/xk6@latest

RUN xk6 build \
    --with github.com/grafana/xk6-dns \
    --with github.com/grafana/xk6-tls \
    --with github.com/phymbert/xk6-sse \
    --output /k6

FROM grafana/k6:1.7.1@sha256:4fd3a694926b064d3491d9b02b01cde886583c4931f1223816e3d9a7bdfa7e0f
COPY --from=builder /k6 /usr/bin/k6
