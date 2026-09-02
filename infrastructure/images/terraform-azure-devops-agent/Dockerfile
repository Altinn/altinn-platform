# renovate: datasource=docker packageName=ghcr.io/altinn/altinn-platform/azure-devops-agent
FROM ghcr.io/altinn/altinn-platform/azure-devops-agent:v1.2.10@sha256:dc5725da20cc4d74eaa4f2e260639f9d56ac4d467f99a656f7e50f687af42408
COPY scripts/install.sh .
RUN ./install.sh

# Wrapper around the base image entrypoint, sets up git authentication with a
# GitHub App installation token before starting the agent
COPY scripts/gh-app-token.sh scripts/entrypoint.sh /azp/
COPY scripts/git-credential-gh-app.sh /usr/local/bin/git-credential-gh-app
RUN chmod +x /azp/gh-app-token.sh /azp/entrypoint.sh /usr/local/bin/git-credential-gh-app

ENTRYPOINT [ "/azp/entrypoint.sh" ]
