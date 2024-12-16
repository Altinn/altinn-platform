# Altinn Azure DevOps Agent Image
Default image used for Altinns self-hosted azure devops agents.

This image is base on the example code from [Azure-Samples/container-apps-ci-cd-runner-tutorial](https://github.com/Azure-Samples/container-apps-ci-cd-runner-tutorial)

The image is maintained by the platform team.

The image is meant to be as small and lightweight as possible so we keep the dependencies at a minimum to reduce the maintenance cost.

If any team needs a custom image they are free to roll their own or extend this, but they will be responsible for maintaining this image.

Example Dockerfile for an image that in addition to what is available in the base image installs netcat:

 ```Dockerfile
 FROM ghcr.io/altinn/altinn-platform/azure-devops-agent:1.0.0
 
+# Switch to root to install additional packages
 USER root
 
+# Install curl and jq while keeping the image size minimal
 RUN apt-get update && apt-get install -y curl jq && \
     apt-get clean && \
     rm -rf /var/lib/apt/lists/*
 
+# Switch back to the runner user for security
 USER runner
