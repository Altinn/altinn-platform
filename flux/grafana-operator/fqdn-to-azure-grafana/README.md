# Grafana FQDN Redirect

Redirect all traffic from a legacy Grafana host to an Azure Managed Grafana host, preserving path and query.

## Variables

- `REDIRECT_GRAFANA_FROM_FQDN`: Source (legacy) Grafana FQDN (no protocol), e.g. `grafana.altinn.cloud`
- `REDIRECT_GRAFANA_TO_FQDN`: Target Azure Managed Grafana FQDN (no protocol), e.g. `altinn-grafana-xyz.eno.grafana.azure.com`

## Behavior

Any HTTPS request to `https://${REDIRECT_GRAFANA_FROM_FQDN}` including arbitrary path and query is permanently (HTTP 301) redirected to:
`https://${REDIRECT_GRAFANA_TO_FQDN}<same path><same query>`

Fragments (`#...`) are never sent to the server and are not part of the redirect (standard HTTP behavior).

## Resources

- Traefik Middleware: `redirect-grafana-fqdn-to-azure-grafana`
- Traefik IngressRoute: `redirect-grafana-fqdn-to-azure-grafana` (entryPoint: `https`, service: `noop@internal`)

## Test

Example:
```
curl -I https://grafana.altinn.cloud/d/abc123/my-dashboard?orgId=1
```
Expect: `HTTP/1.1 301 Moved Permanently` with `Location: https://<target>/d/abc123/my-dashboard?orgId=1`.

## Notes

- Do not include protocol or trailing slash in FQDN variables.
- Change to a temporary redirect (302) by setting `permanent: false` in `middleware.yaml` if doing a staged rollout.