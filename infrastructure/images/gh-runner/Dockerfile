FROM ghcr.io/actions/actions-runner:2.330.0@sha256:ee54ad8776606f29434f159196529b7b9c83c0cb9195c1ff5a7817e7e570dcfe
# for latest release, see https://github.com/actions/runner/releases

USER root

# install additional utilities (curl, jq, unzip, git already in base image)
RUN apt-get update && apt-get install -y wget apt-utils

# Install Azure CLI
RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash

# Install Node.js 20 (required for many GitHub Actions)
# TODO: Upgrade to Node.js 24 before March 2026 (Node 20 EOL: April 2026)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs

# Install GitHub CLI
RUN mkdir -p -m 755 /etc/apt/keyrings && \
    wget -qO- https://cli.github.com/packages/githubcli-archive-keyring.gpg | tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null && \
    chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null && \
    apt-get update && \
    apt-get install -y gh

RUN apt-get clean && \
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
