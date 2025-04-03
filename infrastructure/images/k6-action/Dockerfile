FROM alpine:3.21.3@sha256:a8560b36e8b8210634f77d9f7f9efd7ffa463e380b75e2e74aff4511df3ef88c
COPY --from=golang:1.24-alpine@sha256:7772cb5322baa875edd74705556d08f0eeca7b9c4b5367754ce3f2f00041ccee /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"

ARG KUBECTL_VERSION=1.32.3
ARG KUBESEAL_VERSION=0.29.0
ARG JSONNET_VERSION=0.20.0
ARG K6_VERSION=0.57.0
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

RUN mkdir /tools_download

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

# Install k6
RUN curl -OL "https://github.com/grafana/k6/releases/download/v${K6_VERSION}/k6-v${K6_VERSION}-linux-amd64.tar.gz" &&\
    tar -xvzf k6-v${K6_VERSION}-linux-amd64.tar.gz k6-v${K6_VERSION}-linux-amd64/k6 &&\
    chmod +x k6-v${K6_VERSION}-linux-amd64/k6 && mv k6-v${K6_VERSION}-linux-amd64/k6 /usr/local/bin

RUN mkdir -p /jsonnet/vendor

WORKDIR /jsonnet/vendor

# Download k8s libsonnet library
RUN jb init && \
    jb install github.com/jsonnet-libs/k8s-libsonnet/${K8S_LIBSONNET_VERSION}@main


ADD default_scenarios /actions/generate-k6-manifests/default_scenarios/
ADD infra /actions/generate-k6-manifests/infra/
ADD jsonnet /actions/generate-k6-manifests/jsonnet/

RUN rm -rf /tools_download && rm -rf /var/cache/apk/*

WORKDIR /

CMD ["/bin/bash"]
