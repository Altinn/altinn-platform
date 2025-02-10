FROM alpine:3.21.2

ARG KUBECTL_VERSION=1.32.1
ARG KUBESEAL_VERSION=0.28.0
ARG JSONNET_VERSION=0.20.0
ARG K6_VERSION=0.56.0
ARG JB_VERSION=0.6.0

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
RUN curl -LO "https://dl.k8s.io/release/"v$KUBECTL_VERSION"/bin/linux/amd64/kubectl" &&\
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

WORKDIR /

RUN rm -rf /tools_download && rm -rf /var/cache/apk/*

CMD ["/bin/bash"]
