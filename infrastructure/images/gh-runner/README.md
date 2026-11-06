# Altinn Github Runner Image
Default image used for Altinn's self-hosted Github runners.

This image is maintained by the platform team.

## Extending

This image is meant to be as small and lightweight as possible so we keep the dependencies at a minimum, to reduce the maintenance cost.

If any team needs a custom image they are free to roll their own or extend this, but they will be responsible for maintaining this image.

Example Dockerfile for an image that in addition to what is available in the base image installs curl and jq:

```Dockerfile
FROM ghcr.io/altinn/altinn-platform/gh-runner:v0.10.0 # Make sure to use the latest version 

USER root

RUN apt-get update && apt-get install -y curl jq && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# If you need a WORKDIR command, make sure to set it to the expected WORKDIR in the end.
# WORKDIR /<some_dir>
# <...>
# WORKDIR /home/runner 

USER runner
```

## GitHub API rate limiting

All runners using the same GitHub App share the rate limit of its installation. The entrypoint therefore:

- Retries rate limited (403/429), server (5xx) and network errors, honouring `Retry-After` and `X-RateLimit-Reset`.
  The total wait per request is capped by `GITHUB_API_RETRY_BUDGET` (seconds, default `600`). If the rate limit
  resets later than that, the runner waits out the whole budget before giving up, rather than exiting at once and being
  restarted. The app JWT is signed again before every attempt, so it doesn't expire during long waits.
- Fails the runner with a clear log message when it does not get a token, instead of registering with an empty one.
- Uses `INSTALL_ID` (optional) to request the access token directly, saving one API call per runner.

See [scripts/entrypoint.sh](scripts/entrypoint.sh) for all supported environment variables.

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
