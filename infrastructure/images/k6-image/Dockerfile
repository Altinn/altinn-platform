FROM golang:1.26-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS builder
RUN apk update && apk upgrade && apk add git

RUN go install go.k6.io/xk6/cmd/xk6@latest

RUN xk6 build \
    --with github.com/grafana/xk6-dns \
    --with github.com/grafana/xk6-tls \
    --with github.com/phymbert/xk6-sse \
    --output /k6

FROM grafana/k6:2.1.0@sha256:65c920dc067d5e2e00befbf982af6ad6ad0117034e8b1c65817c7975c52d4669
COPY --from=builder /k6 /usr/bin/k6
