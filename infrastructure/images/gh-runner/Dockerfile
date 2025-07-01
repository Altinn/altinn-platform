FROM ghcr.io/actions/actions-runner:2.325.0@sha256:deb54a88ead0a86beedec6ac949e8b28f77478835b9c6434ccc237390a6e3e4f
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
