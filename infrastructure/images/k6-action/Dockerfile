FROM golang:1.25-alpine@sha256:d3f0cf7723f3429e3f9ed846243970b20a2de7bae6a5b66fc5914e228d831bbb AS builder
# renovate: datasource=github-tags depName=kubectl packageName=kubernetes/kubectl versioning=semver extractVersion=^kubernetes-(?<version>.*)$
ARG KUBECTL_VERSION=1.34.2
# renovate: datasource=github-releases depName=kubeseal packageName=bitnami-labs/sealed-secrets versioning=semver extractVersion=^sealed-secrets-(?<version>.*)$
ARG KUBESEAL_VERSION=0.33.1
# renovate: datasource=github-releases depName=jsonnet packageName=google/jsonnet versioning=semver
ARG JSONNET_VERSION=0.21.0
# renovate: datasource=github-releases depName=jsonnet-bundler packageName=jsonnet-bundler/jsonnet-bundler versioning=semver extractVersion=^v(?<version>.*)$
ARG JB_VERSION=0.6.0
ARG K8S_LIBSONNET_VERSION=1.32

RUN apk --update-cache upgrade && \
    apk add --no-cache \
    bash \
    build-base \
    curl \
    git \
    jq \
    make \
    yq

WORKDIR /tools_download

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" &&\
    chmod +x kubectl && mv kubectl /usr/local/bin

# Install kubeseal
RUN curl -OL "https://github.com/bitnami-labs/sealed-secrets/releases/download/v${KUBESEAL_VERSION}/kubeseal-${KUBESEAL_VERSION}-linux-amd64.tar.gz" &&\
    tar -xvzf kubeseal-${KUBESEAL_VERSION}-linux-amd64.tar.gz kubeseal &&\
    chmod +x kubeseal && mv kubeseal /usr/local/bin

# Install jsonnet and jsonnetfmt
RUN curl -L "https://github.com/google/jsonnet/archive/refs/tags/v${JSONNET_VERSION}.tar.gz" -o "jsonnet-${JSONNET_VERSION}.tar.gz" &&\
    tar -xvzf "jsonnet-${JSONNET_VERSION}.tar.gz" &&\
    cd "jsonnet-${JSONNET_VERSION}" &&\
    make &&\
    chmod +x jsonnet && mv jsonnet /usr/local/bin &&\
    chmod +x jsonnetfmt && mv jsonnetfmt /usr/local/bin

# Install jb
RUN curl -OL "https://github.com/jsonnet-bundler/jsonnet-bundler/releases/download/v${JB_VERSION}/jb-linux-amd64" &&\
    chmod +x jb-linux-amd64 && mv jb-linux-amd64 /usr/local/bin/jb

WORKDIR /jsonnet/vendor

# Download k8s libsonnet library
RUN jb init && \
    jb install github.com/jsonnet-libs/k8s-libsonnet/${K8S_LIBSONNET_VERSION}@main

# Build Generator code
WORKDIR /actions/generate-k6-manifests
COPY actions/generate-k6-manifests .
RUN go build . && \
    chmod +x generate-k6-manifests && \
    mv generate-k6-manifests /usr/local/bin

FROM ghcr.io/altinn/altinn-platform/k6-image:v1.4.2@sha256:1b85365b9c544eeab6c6ca65d81ece5b95cbea79712ea462bc28127f29224c93 AS k6image

# Final image with the needed binaries and helper files
FROM alpine:3.23.0@sha256:51183f2cfa6320055da30872f211093f9ff1d3cf06f39a0bdb212314c5dc7375
COPY --from=builder /usr/local/bin /usr/local/bin
COPY --from=builder /jsonnet/vendor /jsonnet/vendor
COPY --from=k6image /usr/bin/k6 /usr/bin/k6
COPY infrastructure/images/k6-action/default_scenarios /actions/generate-k6-manifests/default_scenarios/
COPY infrastructure/images/k6-action/infra /actions/generate-k6-manifests/infra/
COPY infrastructure/images/k6-action/jsonnet /actions/generate-k6-manifests/jsonnet/

# err:Error loading shared library libstdc++.so.6: No such file or directory (needed by /usr/local/bin/jsonnet)
RUN apk add --no-cache libstdc++
WORKDIR /workspace

CMD ["generate-k6-manifests"]
