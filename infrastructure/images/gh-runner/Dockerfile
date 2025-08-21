FROM ghcr.io/actions/actions-runner:2.328.0@sha256:db0dcae6d28559e54277755a33aba7d0665f255b3bd2a66cdc5e132712f155e0
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
