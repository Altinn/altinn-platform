FROM golang:1.26-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039  AS builder
# renovate: datasource=github-tags depName=kubectl packageName=kubernetes/kubectl versioning=semver extractVersion=^kubernetes-(?<version>.*)$
ARG KUBECTL_VERSION=1.36.0
# renovate: datasource=github-releases depName=kubeseal packageName=bitnami-labs/sealed-secrets versioning=semver extractVersion=^sealed-secrets-(?<version>.*)$
ARG KUBESEAL_VERSION=0.36.6
# renovate: datasource=github-releases depName=jsonnet packageName=google/jsonnet versioning=semver
ARG JSONNET_VERSION=0.22.0
# renovate: datasource=github-releases depName=jsonnet-bundler packageName=jsonnet-bundler/jsonnet-bundler versioning=semver extractVersion=^v(?<version>.*)$
ARG JB_VERSION=0.6.0
ARG K8S_LIBSONNET_VERSION=1.33

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
RUN go mod download
RUN go build . && \
    chmod +x generate-k6-manifests && \
    mv generate-k6-manifests /usr/local/bin

FROM ghcr.io/altinn/altinn-platform/k6-image:v1.7.0@sha256:5e88f2669758eac8f5a58efe51654c585ea5435d9787d6ccf54e989198395f10 AS k6image

# Final image with the needed binaries and helper files
FROM alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
COPY --from=builder /usr/local/bin /usr/local/bin
COPY --from=builder /jsonnet/vendor /jsonnet/vendor

COPY --from=k6image /usr/bin/k6 /usr/bin/k6

COPY infrastructure/images/k6-action/default_scenarios /actions/generate-k6-manifests/default_scenarios/
COPY infrastructure/images/k6-action/infra /actions/generate-k6-manifests/infra/
COPY infrastructure/images/k6-action/jsonnet /actions/generate-k6-manifests/jsonnet/

# err:Error loading shared library libstdc++.so.6: No such file or directory (needed by /usr/local/bin/jsonnet)
RUN apk add --no-cache libstdc++ git
WORKDIR /workspace

CMD ["generate-k6-manifests"]
