# renovate: datasource=docker packageName=ghcr.io/altinn/altinn-platform/azure-devops-agent
FROM ghcr.io/altinn/altinn-platform/azure-devops-agent:v1.4.0@sha256:d02acc6e8a62dc6f0a7a64fce1b60af1ef235f2fc716746d273259c5823a2634
COPY scripts/install.sh .
RUN ./install.sh

# Wrapper around the base image entrypoint, sets up git authentication with a
# GitHub App installation token before starting the agent
COPY scripts/gh-app-token.sh scripts/entrypoint.sh /azp/
COPY scripts/git-credential-gh-app.sh /usr/local/bin/git-credential-gh-app
RUN chmod +x /azp/gh-app-token.sh /azp/entrypoint.sh /usr/local/bin/git-credential-gh-app

ENTRYPOINT [ "/azp/entrypoint.sh" ]
