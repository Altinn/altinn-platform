# Altinn Github Runner Image
Default image used for Altinns self-hosted github runners.

This image is maintained by the platform team.

## Extending

This image is ment to be as small and leightweight as possible so we keep the dependencies at a minum, to reduce the maintenance cost.

If any team needs a custom image they are free to roll their own or extend this, but they will be responsible for maintaining this image.

Example Dockerfile for an image that in addition to what is available in the base image installs netcat:

```Dockerfile
FROM ghcr.io/altinn/altinn-platform-gh-runner-base:1.0.0 ##TODO: Add actual image name when available

USER root

RUN apt-get update && apt-get install -y curl jq && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

USER runner
```

## Maintenance
Renovate is enabled on this repository and will automatically create a PR when there is a new version of the base image.

### New trivy alerts
This image is set up with Trivy to scan for vulnerabilities. If any vulnerabilities are found, the workflow will fail.

#### Managing Vulnerabilities
1. **False Positives**: If an alert is a false positive, add the CVE ID to `.trivyignore`
2. **Accepted Risks**: For known risks that have been assessed and accepted:
   - Add the CVE ID to `.trivyignore`
   - Add a comment above the CVE explaining:
     - Why the risk is acceptable
     - Any mitigating controls in place
     - When the decision should be reviewed