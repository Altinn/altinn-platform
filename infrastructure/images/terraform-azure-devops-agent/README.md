# Altinn Terraform Azure DevOps Agent Image

Image maintained by the platform team which installs some standard software that is used by our Terraform pipelines

## Maintenance
Renovate is enbaled on this repo so it should issue a PR when there is a new version of the base image.
If quicker turnaround is needed update the `Dockerfile`

### Additional software installed

#### kubectl
This should be updated to the latest stable release once a month or when some other update is made to the image.

To update the kubectl version get the latest stable release with `curl -L -s https://dl.k8s.io/release/stable.txt` and update the variable KUBECTL_VERSION in the top of the `scripts/install.sh` file.

### New trivy alerts
This image is setup with trivy to scan for vulnerabilities. If there are any found the workflow will fail.
If the alert is a false positive or a risk we are willing to live with the CVE ID can be added to the `.trivyignore`. A comment above the CVE should explain why the ignore is added.