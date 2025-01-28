# Altinn Terraform Azure DevOps Agent Image

Image maintained by the platform team which installs some standard software that is used by our Terraform pipelines

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