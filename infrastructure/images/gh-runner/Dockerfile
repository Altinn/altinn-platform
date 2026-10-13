FROM ghcr.io/actions/actions-runner:2.338.0@sha256:4ffadc0002b2581327e06101fc8c06cd189232baf79fe561fac9caeb76f5e807
# for latest release, see https://github.com/actions/runner/releases

USER root

# install additional utilities (curl, jq, unzip, git already in base image)
# and pick up Ubuntu security fixes newer than the base image (e.g. openssl)
RUN apt-get update && \
    apt-get upgrade -y --no-install-recommends && \
    apt-get install -y --no-install-recommends wget uuid-runtime make xz-utils

# Install Azure CLI
RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash

# Install Node.js 24 (required for many GitHub Actions)
RUN mkdir -p /etc/apt/keyrings && \
    curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg && \
    echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_24.x nodistro main" | tee /etc/apt/sources.list.d/nodesource.list && \
    apt-get update && \
    apt-get install -y nodejs && \
    corepack enable

# Install GitHub CLI
RUN mkdir -p -m 755 /etc/apt/keyrings && \
    wget -qO- https://cli.github.com/packages/githubcli-archive-keyring.gpg | tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null && \
    chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null && \
    apt-get update && \
    apt-get install -y gh

# Install yq (the Go implementation from mikefarah/yq, not the unrelated python yq in apt)
# renovate: datasource=github-releases depName=yq packageName=mikefarah/yq versioning=semver
ARG YQ_VERSION=4.54.1
RUN curl -fsSL "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_$(dpkg --print-architecture)" -o /usr/local/bin/yq && \
    chmod 0755 /usr/local/bin/yq && \
    yq --version

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
