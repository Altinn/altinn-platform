FROM alpine:latest
# TODO: Create a separate image and use that one as the base
# Have automated tests to validate that the new versions didn't break anything.
ARG KUBESEAL_VERSION=0.28.0
ARG KUBECFG_VERSION=0.35.2
ARG K6_VERSION=0.56.0

RUN apk add --no-cache curl jq yq make build-base git bash

RUN mkdir tools_download

RUN cd tools_download \
    && curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && chmod +x kubectl && mv kubectl /usr/local/bin

RUN cd tools_download \
    && KUBESEAL_VERSION=$(curl -s https://api.github.com/repos/bitnami-labs/sealed-secrets/tags | jq -r '.[0].name' | cut -c 2-) \
    && if [ -z "$KUBESEAL_VERSION" ]; then echo "Failed to fetch the latest KUBESEAL_VERSION" && exit 1; fi \
    && curl -OL "https://github.com/bitnami-labs/sealed-secrets/releases/download/v${KUBESEAL_VERSION}/kubeseal-${KUBESEAL_VERSION}-linux-amd64.tar.gz" \
    && tar -xvzf kubeseal-${KUBESEAL_VERSION}-linux-amd64.tar.gz kubeseal \
    && chmod +x kubeseal && mv kubeseal /usr/local/bin

RUN cd tools_download \
    && JSONNET_VERSION=$(curl -s https://api.github.com/repos/google/jsonnet/tags | jq -r '.[0].name' | cut -c 2-) \
    && if [ -z "$JSONNET_VERSION" ]; then echo "Failed to fetch the latest JSONNET_VERSION" && exit 1; fi \
    && curl -L "https://github.com/google/jsonnet/archive/refs/tags/v${JSONNET_VERSION}.tar.gz" -o "jsonnet-${JSONNET_VERSION}.tar.gz" \
    && tar -xvzf "jsonnet-${JSONNET_VERSION}.tar.gz" \
    && cd "jsonnet-${JSONNET_VERSION}" \
    && make \
    && chmod +x jsonnet && mv jsonnet /usr/local/bin \
    && chmod +x jsonnetfmt && mv jsonnetfmt /usr/local/bin

RUN cd tools_download \
    && JB_VERSION=$(curl -s https://api.github.com/repos/jsonnet-bundler/jsonnet-bundler/tags | jq -r '.[0].name' | cut -c 2-) \
    && if [ -z "$JB_VERSION" ]; then echo "Failed to fetch the latest JB_VERSION" && exit 1; fi \
    && curl -OL "https://github.com/jsonnet-bundler/jsonnet-bundler/releases/download/v${JB_VERSION}/jb-linux-amd64" \
    && chmod +x jb-linux-amd64 && mv jb-linux-amd64 /usr/local/bin/jb

RUN cd tools_download \
    && K6_VERSION=$(curl -s https://api.github.com/repos/grafana/k6/tags | jq -r '.[0].name' | cut -c 2-) \
    && if [ -z "$K6_VERSION" ]; then echo "Failed to fetch the latest K6_VERSION" && exit 1; fi \
    && curl -OL "https://github.com/grafana/k6/releases/download/v${K6_VERSION}/k6-v${K6_VERSION}-linux-amd64.tar.gz" \
    && tar -xvzf k6-v${K6_VERSION}-linux-amd64.tar.gz k6-v${K6_VERSION}-linux-amd64/k6 \
    && chmod +x k6-v${K6_VERSION}-linux-amd64/k6 && mv k6-v${K6_VERSION}-linux-amd64/k6 /usr/local/bin

ADD generate.sh /generate.sh

CMD ["/generate.sh"]
