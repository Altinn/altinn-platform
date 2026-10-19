- Feature Name: headscale_acl_sync
- Title: Headscale ACL policy and VPN access from Git
- Start Date: 2026-10-18
- RFC PR: [altinn/altinn-platform#4233](https://github.com/Altinn/altinn-platform/pull/4233)
- Implementation PRs, in merge order:
  1. [dis-way/adminservices#414](https://github.com/dis-way/adminservices/pull/414): Terraform root `headscale-access` and its workflow
  2. [dis-way/gitops-manifests#2128](https://github.com/dis-way/gitops-manifests/pull/2128): `oci/headscale/acl-sync` (ExternalSecret and CronJob)
  3. [dis-way/adminservices#415](https://github.com/dis-way/adminservices/pull/415): migration in `admin-prod-aks-rg` and the Headplane `it_admin` role
- Github Issue: None.
- Product/Category: VPN (Headscale)
- State: **REVIEW** (possible states are: **REVIEW**, **ACCEPTED** and **REJECTED**)

# Summary
[summary]: #summary

Git is the only source of truth for the Headscale Access Control List (ACL) policy and for VPN access.
One pull request grants VPN access and changes the ACL rules.
Terraform renders the policy, with the group members, into Azure Key Vault.
External Secrets Operator (ESO) copies the policy to a Kubernetes Secret, and a CronJob applies it to Headscale.
The CronJob overwrites all other changes, for example edits in Headplane.

# Motivation
[motivation]: #motivation

The current process has these problems:

- The ACL policy is only in the Headscale database. We cannot see who changed a rule, when, or why.
- If we lose the Headscale database, we lose the policy.
- A [VPN access request](../.github/ISSUE_TEMPLATE/vpn-access-request.yml) has two manual steps in two places: add the user to the Entra ID group `headscale-vpn-access` with Terraform, and add the user to ACL groups in Headscale.
- The Terraform code for `headscale-vpn-access` is in the same root as the AKS cluster. Each VPN access request makes a plan of the full cluster.

The expected outcome:

- One pull request grants VPN access and changes the ACL policy.
- Git shows every change to the policy and to the group members.
- A change applies without a restart of Headscale.
- A new Headscale database gets the policy automatically.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation

All files are in a new Terraform root `tf/adminservices-prod/headscale-access/` in [`dis-way/adminservices`](https://github.com/dis-way/adminservices).
The workflow `headscale-access.yml` runs only for this root.

**Grant VPN access to a new user.** The admin adds the object ID of the user to a group in `terraform.tfvars`:

```hcl
headscale_groups = {
  team-core-prod = [
    "272b7fb3-d973-4046-b0db-1b5a8c9f5e8e", # first.last@digdir.no
  ]
  # Keep a group that has no members as an empty list. The ACL rules still use it.
  team-core-test = []
}
```

After the merge, the user is a member of `headscale-vpn-access` and of `group:team-core-prod` in the ACL policy.
The admin does not edit Headscale or Entra ID.
A user can be in more than one group.

**Change a rule.** The admin changes `acl.hujson.tftpl` in the same root:

```hujson
{
  // Terraform writes the groups from headscale_groups, one member on each line.
  "groups": {
%{ for name, members in groups ~}
    "group:${name}": [
%{ for member in members ~}
      "${member}",
%{ endfor ~}
    ],
%{ endfor ~}
  },

  "acls": [
    {"action": "accept", "src": ["group:team-core-prod"], "dst": ["dis-core-prod-aks:443"]},
  ],
}
```

The template also contains `hosts`, `tagOwners` and `ssh`. The admin edits them by hand.

**Remove a group.** Remove the key from `headscale_groups`, and remove all uses of `group:<name>` from the template in the same pull request.
If a rule uses a group that does not exist, `headscale policy check` fails.

**Headplane.** Admins get the [Headplane role](https://headplane.net/features/sso) IT Admin. This role can manage machines and users, but it cannot change ACLs.

**What is lost.** An edit with the Headscale API, or by an Owner or Admin in Headplane, lasts until the next CronJob run, at most 5 minutes.
The CronJob writes a warning with the difference to the job log before it overwrites the edit.

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

## Flow

```
Pull request → headscale-access.yml → Terraform
  → Entra ID group headscale-vpn-access
  → Key Vault secret headscale-acl → ESO → Secret in flux-system
  → CronJob → kubectl exec → headscale policy set
```

## Terraform

| Resource | Use |
|---|---|
| `azuread_group.headscale_vpn_access` and members | The members are all users in `headscale_groups`. Only these users can log in to Headscale. |
| `azuread_app_role_assignment.headscale_vpn_access` | Assigns the group to the Headscale OIDC app. The app registration stays in `admin-prod-aks-rg`, and `data.azuread_service_principal` gets it. |
| `data.azuread_user` for each member | Gets `mail` and `user_principal_name`. The workflow identity needs the Microsoft Graph permission `User.Read.All`. |
| `azurerm_key_vault_secret.headscale_acl` | The rendered policy: `templatefile("acl.hujson.tftpl", { groups = local.groups })`. It is in the dis-system Key Vault of admin-prod-aks (`module.dis_system_kv` in `admin-prod-aks-rg`). The workflow identity already has the role Key Vault Secrets Officer on it. |
| `output.headscale_acl` | The rendered policy as a non-sensitive output. The plan shows a readable diff, because the value of a Key Vault secret is sensitive. |

The ACL groups come directly from `headscale_groups`. We do not create an Entra ID group for each ACL group.

Some users are in more than one group. The member resources and the user lookups use the set of unique object IDs as keys:

```hcl
locals {
  headscale_members = toset(flatten(values(var.headscale_groups)))

  # Group name without the "group:" prefix → user identifiers. The template adds the prefix.
  groups = {
    for name, ids in var.headscale_groups :
    name => [for id in ids : local.headscale_identifier[id]]
  }
}

resource "azuread_group_member" "headscale_vpn_access" {
  for_each         = local.headscale_members
  group_object_id  = azuread_group.headscale_vpn_access.object_id
  member_object_id = each.value
}
```

`local.headscale_identifier` maps each object ID to the identifier in [User identifier](#user-identifier).

The template writes each group member on its own line.
With `jsonencode`, all groups would be on one line, and the plan would show one long changed line for each change.

The rendered policy is approximately 12 KB. The maximum size of a Key Vault secret value is 25 KB.

## User identifier

Headscale stores OIDC users as `name@digdir.no`. The users are guest users (Entra ID B2B).
The `user_principal_name` of a guest user has the format `first.last_digdir.no#EXT#@<tenant>.onmicrosoft.com`.

1. If `mail` is not empty, use `mail`.
2. Else, use the part of `user_principal_name` before `#EXT#`, and replace the last `_` with `@`.
3. Change the identifier to lowercase.
4. A `precondition` fails the plan if the domain is not `digdir.no`.

## Validation

The workflow runs these steps on each pull request:

1. `terraform plan`.
2. Write `output.headscale_acl` from the plan to a file.
3. Diff the file against the `headscale_acl` output in the stored state, from the last apply. `prior_state` in the plan JSON cannot be used, because its outputs are already evaluated with the new configuration.
4. Write the diff to one PR comment, which the workflow updates on each push. Write the diff and the full policy to the workflow summary.
5. Run `headscale policy check -f` on the file. The workflow reads the Headscale version from `oci/headscale/deployment.yaml` in gitops-manifests.

In Headscale v0.29, `policy check` is a gRPC call to a running server by default.
The workflow uses `--force --bypass-grpc-and-access-database-directly` with a minimal configuration and an empty database.
This finds HuJSON errors and groups, hosts and tags that do not exist.
The CronJob checks the policy again against the live users and nodes before it applies it.

The check is a required status check. An invalid policy does not get to Key Vault.

## Kubernetes

The resources are in the layer `oci/headscale/acl-sync` in [`dis-way/gitops-manifests`](https://github.com/dis-way/gitops-manifests).

| Resource | Namespace | Use |
|---|---|---|
| ExternalSecret `headscale-acl` | `flux-system` | Copies the Key Vault secret from `dis-system-store` to the Secret `headscale-acl`. `refreshInterval: 5m`. |
| ServiceAccount, Roles and RoleBindings `headscale-acl-apply` | `headscale`, `flux-system` | `pods/exec` on Headscale, and `get` on only the Secret `headscale-acl` in `flux-system`. |
| CiliumNetworkPolicy `headscale-acl-apply-egress-apiserver` | `headscale` | Egress to the API server only. DNS is already open, and everything else stays denied. |
| ConfigMap `headscale-acl-apply` | `headscale` | The script of the CronJob. |
| CronJob `headscale-acl-apply` | `headscale` | Runs every 5 minutes with `concurrencyPolicy: Forbid`. |

`dis-system-store` is a namespaced SecretStore in `flux-system`, so the ExternalSecret and its Secret are there.
The CronJob reads the Secret with `kubectl`, so it does not need a SecretStore in `headscale`.

The flux configuration `headscale` deploys the layer as its own Kustomization, `headscale-acl-sync` (path `./acl-sync/`, `dependsOn: headscale`, `wait: false`).
Flux deploys it together with Headscale, so a new Headscale database gets the policy within 5 minutes of the first deploy.
With `wait: false`, a failing ExternalSecret does not block the Headscale Kustomization.

**Connection.** The CronJob runs the `headscale` CLI in the Headscale pod with `kubectl exec -i deploy/headscale -c headscale -- headscale ...` over the unix socket.
The CronJob does not need an API key, and the gRPC API stays bound to `127.0.0.1`.
The remote CLI (`HEADSCALE_CLI_ADDRESS`) is not possible. It always connects to gRPC with TLS, and `HEADSCALE_CLI_INSECURE` only skips the certificate check.
Headscale serves gRPC with TLS only when it terminates TLS for the whole server, and Traefik terminates TLS here.
Do not use `--bypass-grpc-and-access-database-directly` in the cluster. It opens the SQLite database while Headscale uses it.

**Image.** The CronJob uses `chainguard/kubectl:latest-dev` through the `altinncr.azurecr.io` mirror. It has a low CVE count and contains `sh`, `cmp`, `diff` and `base64`.
The free tier has only `latest` tags, so the image is pinned by digest. A Renovate regex manager updates the digest.
`bitnami/kubectl` gets no updates after 2025-08-28, and `registry.k8s.io/kubectl` has no shell.

**Script.**

```sh
set -eu
headscale() {
  kubectl exec -i -n headscale deploy/headscale -c headscale -- headscale "$@"
}
kubectl get secret -n flux-system headscale-acl -o jsonpath='{.data.acl\.hujson}' | base64 -d >/tmp/acl.hujson
# policy get prints the stored policy followed by a newline.
{ cat /tmp/acl.hujson; echo; } >/tmp/want.hujson
headscale policy get >/tmp/live.hujson 2>/dev/null || : >/tmp/live.hujson
if cmp -s /tmp/live.hujson /tmp/want.hujson; then
  echo "policy in sync"; exit 0
fi
echo "WARNING: live policy differs from Git, overwriting"
diff -u /tmp/live.hujson /tmp/want.hujson || true
headscale policy check -f /dev/stdin </tmp/acl.hujson
headscale policy set -f /dev/stdin </tmp/acl.hujson
```

If `policy check` or `policy set` fails, the live policy does not change and the run fails.

**Pod.** User 65532, `readOnlyRootFilesystem: true` and `capabilities.drop: [ALL]`, with an `emptyDir` on `/tmp`.
The pod is not in the Linkerd mesh, because it talks only to the API server.

## Headplane roles

The Entra ID app role for Headplane has the value `it_admin`. Headplane reads it from `oidc.role_claim` at each login, so the role is in Terraform.
IT Admin cannot change ACLs. To edit the ACL in Headplane, an admin must first give a user a role with ACL access.
This extra step reminds admins to make the change in Git. The CronJob removes the edit within 5 minutes.

The first user who logs in to Headplane gets the role Owner, and `role_claim` cannot change it. We accept this, because the CronJob also removes edits by the Owner.

## Emergency edit

Use this procedure only if a change cannot wait for a pull request.

1. Validate the policy: `kubectl exec -i -n headscale deploy/headscale -c headscale -- headscale policy check -f /dev/stdin < acl.hujson`.
2. Write it to Key Vault: `az keyvault secret set --vault-name admin-prod-4tnxzq --name headscale-acl --file acl.hujson`.
3. Start ESO: `kubectl annotate externalsecret headscale-acl -n flux-system force-sync=$(date +%s) --overwrite`.
4. Start the job: `kubectl create job -n headscale --from=cronjob/headscale-acl-apply headscale-acl-apply-manual`.
5. Open a pull request with the same change immediately.

The next `terraform apply` in the root writes the policy from Git to Key Vault. If step 5 is not done, the apply removes the emergency edit.

## Migration

1. Export the live policy with `headscale policy get`. Make it into `acl.hujson.tftpl`, and move the ACL group members into `headscale_groups`.
   Keep the groups that have no members as empty lists.
   On 2026-10-19, the 49 users in the ACL groups were the same 49 users as in `headscale_vpn_access_members`. The migration does not change who can log in.
2. Move `azuread_group.headscale_vpn_access`, its members and `azuread_app_role_assignment.headscale_vpn_access` from `admin-prod-aks-rg` to `headscale-access`. Use `removed` blocks with `destroy = false` in the old root, and `import` blocks in the new root.
   The members need one import for each object ID. Use `for_each` on the `import` block with the ID format `<group object ID>/member/<member object ID>`.
3. The OIDC app registration stays in `admin-prod-aks-rg`. The new root gets its service principal with `data.azuread_service_principal`.
4. The flux configuration `headscale` in `admin-prod-aks-rg` sets `HEADSCALE_APP_ALLOWED_GROUP` from `azuread_group.headscale_vpn_access.object_id`.
   Change it to `data.azuread_group.headscale_vpn_access.object_id` in the same change as the `removed` block. The object ID does not change.
5. Check that no plan shows a destroy. The plan of `admin-prod-aks-rg` updates the flux configuration `headscale` in place: it adds the Kustomization `headscale-acl-sync` and sets the Kustomization sync intervals to 1 hour. `HEADSCALE_APP_ALLOWED_GROUP` does not change.

Merge order:

1. `headscale-access` imports the group, its members and the app role assignment, and writes `headscale-acl` to Key Vault.
   Until the next step is applied, both states manage the members.
2. gitops-manifests releases `oci/headscale` with `acl-sync` to `prod_ring1`.
3. `admin-prod-aks-rg` drops the resources with `removed` blocks and adds the Kustomization `headscale-acl-sync`.
   If this step is applied before step 2, `./acl-sync/` does not exist in the artifact, and `waitForReconciliation` fails the apply.

The first CronJob run always overwrites the live policy and writes a large diff to the job log.
The live policy has different formatting, comments and trailing white space from the rendered template. This is expected.
Compare the diff with the export from step 1 to make sure that only formatting changed.

# Drawbacks
[drawbacks]: #drawbacks

- The Owner in Headplane, or a user that an admin gives ACL access, can still edit the ACL policy. The CronJob removes the edit within 5 minutes.
- An emergency edit is lost if nobody adds it to Git before the next `terraform apply`.
- A change takes up to approximately 15 minutes: the apply, the ESO refresh and the CronJob schedule.
- The Terraform identity of the workflow needs permission to write Entra ID groups and Key Vault secrets, and to read users (Microsoft Graph `User.Read.All`, granted on 2026-10-19).
- The CronJob has `pods/exec` on Headscale. This is full control of the tailnet, the same as the API key that Headplane uses.

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

If we do not do this, the ACL policy stays outside Git, and admins copy group membership by hand.

| Alternative | Why we do not choose it |
|---|---|
| Two-way sync between Git and the live policy | Keeps edits from Headplane, but needs write-back to Git, conflict handling and a GitHub App. Approximately 15 days of work, compared with approximately 5 days. |
| `policy.mode: file` with a Secret volume and a `SIGHUP` sidecar | Headplane becomes read-only, so no edits are lost. But an invalid policy file, for example from an emergency edit, stops Headscale at the next restart. `policy set` rejects an invalid policy before it changes the live policy. |
| `policy.mode: file` with a restart for each change | The control plane is not available during each restart. |
| Group members from Microsoft Graph | Needs a Graph client, a workload identity and admin consent. Not necessary when Terraform manages the members. |
| An Entra ID group for each ACL group | More objects in Entra ID, and the members are still in Terraform. |
| OIDC groups in Headscale ([juanfont/headscale#3216](https://github.com/juanfont/headscale/pull/3216)) | Not merged. Groups update only at login, so a removed user keeps access until the next login. |
| CronJob with the remote CLI over gRPC | The remote CLI always uses TLS, and Headscale cannot serve gRPC with TLS behind Traefik. Opening gRPC would also undo the loopback-only binding. |
| CronJob with the HTTP API (`GET`/`PUT /api/v1/policy`, `POST /api/v1/policy/check`) | Works, but needs an API key in Key Vault, with rotation, and an image with `curl` and `jq`. |
| Sidecar in the Headscale pod over the unix socket | No API key, but no Job history for failed runs, and adding it restarts Headscale. |

# Prior art
[prior-art]: #prior-art

- [tailscale/gitops-acl-action](https://github.com/tailscale/gitops-acl-action) uses the same model for Tailscale. Git wins, and the action writes a warning when an admin changed the policy in the admin console.
- [YouSysAdmin/headscale-pf](https://github.com/YouSysAdmin/headscale-pf) writes group members from an identity provider into a HuJSON policy template. This RFC does the same with `templatefile` in Terraform.

# Unresolved questions
[unresolved-questions]: #unresolved-questions

None. The questions from the review are resolved:

- Key Vault: the dis-system Key Vault of admin-prod-aks. The workflow identity has Key Vault Secrets Officer on it.
- `headscale policy get` returns the stored policy followed by one newline. The script adds the newline before `cmp`. Tested on v0.29.5.
- gRPC: `grpc_allow_insecure` is `false`, and the CronJob does not use gRPC over the network. See **Connection** in [Kubernetes](#kubernetes).
- API key rotation: not needed. The CronJob uses the unix socket.

# Future possibilities
[future-possibilities]: #future-possibilities

- Change the VPN access request template: one pull request replaces the manual steps.
- Manage `tagOwners` and `hosts` from Terraform variables.
- Add a Grafana alert for a failed CronJob run and for an overwritten edit.
