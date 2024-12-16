# Altinn Azure Decops Agent Image
Default image used for Altinns self-hosted azure devops agents.

This image is base on the example code from https://github.com/Azure-Samples/container-apps-ci-cd-runner-tutorial

The image is maintained by the platform team.

The image is ment to be as small and leightweight as possible so we keep the dependencies at a minum, to reduce the maintenance cost.

If any team needs a custom image they are free to roll their own or extend this, but they will be responsible for maintaining this image.

Example Dockerfile for an image that in addition to what is available in the base image installs netcat:

```Dockerfile
FROM ghcr.io/altinn/altinn-platform/azuer-devops-agent:1.0.0

USER root

RUN apt-get update && apt-get install -y curl jq && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

USER runner
```
