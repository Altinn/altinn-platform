FROM golang:1.26-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS builder
RUN apk update && apk upgrade && apk add git

RUN go install go.k6.io/xk6/cmd/xk6@latest

RUN xk6 build \
    --with github.com/grafana/xk6-dns \
    --with github.com/grafana/xk6-tls \
    --with github.com/phymbert/xk6-sse \
    --output /k6

FROM grafana/k6:2.3.0@sha256:9c2dee7f8ed74d317e4027c06a10f169b625638189de8d4555d0b3486a5aeb34
COPY --from=builder /k6 /usr/bin/k6

USER root
RUN apk upgrade --no-cache
USER k6
