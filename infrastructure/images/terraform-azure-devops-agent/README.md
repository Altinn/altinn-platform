# Altinn Terraform Azure DevOps Agent Image

Image maintained by the platform team which installs some standard software that is used by our Terraform pipelines

## GitHub App authentication for git

The image wraps the base image entrypoint with `scripts/entrypoint.sh`. If the GitHub App environment variables below are set, the wrapper requests an installation access token and configures git to use it for `https://github.com` before starting the Azure Pipelines agent. This lets pipelines clone private repositories and use private Terraform modules sourced over https without a PAT.

| Variable | Required | Description |
| --- | --- | --- |
| `APP_ID` | yes | The GitHub app's ID |
| `APP_PRIVATE_KEY` | yes | The content of the GitHub app's private key in PEM format |
| `APP_LOGIN` | yes | The login name (org) the app is installed on, e.g. `Altinn` |
| `GITHUB_HOST` | no | Defaults to `github.com` |

If none of `APP_ID`, `APP_PRIVATE_KEY` and `APP_LOGIN` are set, the wrapper does nothing and the agent starts as before. If only some of them are set, the container fails on startup so the misconfiguration is not silently ignored.

Notes:
* The token is written to `/azp/.github_token` (mode 0600) and handed to git by the `git-credential-gh-app` credential helper, so it never ends up in the git config.
* `APP_PRIVATE_KEY` is unset before the agent is started, so it is not part of the environment of the pipeline jobs.
* The token is requested once, when the container starts. Installation access tokens are valid for one hour, which covers a job on an agent started with `--once`.
* Only https remotes are authenticated. `git@github.com:` remotes are left alone and still need an ssh key.

## Maintenance
Renovate is enabled on this repository and will automatically create a PR when there is a new version of the base image.
If quicker turnaround is needed update the `Dockerfile`

### Additional software installed

#### kubectl
This should be updated to the latest stable release once a month or when some other update is made to the image.

To update the kubectl version get the latest stable release with `curl -L -s https://dl.k8s.io/release/stable.txt` and update the variable KUBECTL_VERSION in the top of the `scripts/install.sh` file.

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