FROM ghcr.io/actions/actions-runner:2.330.0@sha256:25e5339f41c8511899dff6d9d1ee3d29d0aaca085f24b967f486e153e4368cc6
# for latest release, see https://github.com/actions/runner/releases

USER root

# install curl and jq
RUN apt-get update && apt-get install -y curl jq && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY scripts/entrypoint.sh ./entrypoint.sh
COPY scripts/app-token.sh ./app-token.sh
COPY scripts/token.sh ./token.sh
RUN chmod +x ./entrypoint.sh && \
    chmod +x ./app-token.sh && \
    chmod +x ./token.sh && \
    mkdir /_work && \
    chown runner:docker \
    ./entrypoint.sh \
    ./app-token.sh \
    ./token.sh \
    /_work

USER runner

ENTRYPOINT ["./entrypoint.sh"]
