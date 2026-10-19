- Feature Name: headscale_acl_sync
- Title: Headscale ACL policy and VPN access from Git
- Start Date: 2026-10-18
- RFC PR: [altinn/altinn-platform#4233](https://github.com/Altinn/altinn-platform/pull/4233)
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
  → Key Vault secret headscale-acl → ESO → Kubernetes Secret → CronJob → headscale policy set
```

## Terraform

| Resource | Use |
|---|---|
| `azuread_group.headscale_vpn_access` and members | The members are all users in `headscale_groups`. Only these users can log in to Headscale. |
| `azuread_app_role_assignment.headscale_vpn_access` | Assigns the group to the Headscale OIDC app. The app registration stays in `admin-prod-aks-rg`, and `data.azuread_service_principal` gets it. |
| `data.azuread_user` for each member | Gets `mail` and `user_principal_name`. |
| `azurerm_key_vault_secret.headscale_acl` | The rendered policy: `templatefile("acl.hujson.tftpl", { groups = local.groups })`. |
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
3. Run `headscale policy check -f` on the file with the Headscale version of the cluster.

The check is a required status check. An invalid policy does not get to Key Vault.

## Kubernetes

| Resource | Use |
|---|---|
| ExternalSecret `headscale-acl` | Copies the Key Vault secret to a Kubernetes Secret. `refreshInterval: 5m`. |
| ExternalSecret `headscale-cli-apikey` | Gets the Headscale API key from Key Vault. |
| ConfigMap `headscale-acl-apply` | The script of the CronJob. |
| CronJob `headscale-acl-apply` | Runs every 5 minutes with `concurrencyPolicy: Forbid`. |

These resources are in the same Kustomization as the Headscale Deployment, in `manifests/infra/headscale`.
Flux deploys them together with Headscale. A new Headscale database gets the policy within 5 minutes of the first deploy.

**Image.** The CronJob uses `ghcr.io/juanfont/headscale:<version>-debug`.
Headscale publishes this image for each release. It contains the `headscale` binary and a busybox shell.
The CronJob and the Deployment use the same version. One Renovate group updates the two tags in one pull request.

**Connection.** The CronJob connects to `headscale.headscale.svc:50443` with `HEADSCALE_CLI_ADDRESS` and `HEADSCALE_CLI_API_KEY`.
Do not use `--bypass-server-and-access-database-directly`. It opens the SQLite database while Headscale uses it.

**Script.**

```sh
set -eu
headscale policy get > /tmp/live.hujson 2>/dev/null || : > /tmp/live.hujson
if cmp -s /tmp/live.hujson /policy/acl.hujson; then
  echo "policy in sync"; exit 0
fi
echo "WARNING: live policy differs from Git, overwriting"
diff -u /tmp/live.hujson /policy/acl.hujson || true
headscale policy check -f /policy/acl.hujson
headscale policy set -f /policy/acl.hujson
```

If `policy set` fails, the live policy does not change and the run fails.

**Pod.** The same security context as Headscale: user 65532, `readOnlyRootFilesystem: true` and `capabilities.drop: [ALL]`.
The pod mounts an `emptyDir` on `/tmp`, the Secret `headscale-acl` on `/policy`, and the ConfigMap with the script.

## Headplane roles

The Entra ID app role for Headplane has the value `it_admin`. Headplane reads it from `oidc.role_claim` at each login, so the role is in Terraform.
IT Admin cannot change ACLs. To edit the ACL in Headplane, an admin must first give a user a role with ACL access.
This extra step reminds admins to make the change in Git. The CronJob removes the edit within 5 minutes.

The first user who logs in to Headplane gets the role Owner, and `role_claim` cannot change it. We accept this, because the CronJob also removes edits by the Owner.

## Emergency edit

Use this procedure only if a change cannot wait for a pull request.

1. Validate the policy with `headscale policy check -f acl.hujson`.
2. Write it to Key Vault: `az keyvault secret set --vault-name <vault> --name headscale-acl --file acl.hujson`.
3. Start ESO: `kubectl annotate externalsecret headscale-acl -n headscale force-sync=$(date +%s) --overwrite`.
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
5. Check that no plan shows a destroy, and that the plan for the flux configuration `headscale` shows no change.

The first CronJob run always overwrites the live policy and writes a large diff to the job log.
The live policy has different formatting, comments and trailing white space from the rendered template. This is expected.
Compare the diff with the export from step 1 to make sure that only formatting changed.

# Drawbacks
[drawbacks]: #drawbacks

- The Owner in Headplane, or a user that an admin gives ACL access, can still edit the ACL policy. The CronJob removes the edit within 5 minutes.
- An emergency edit is lost if nobody adds it to Git before the next `terraform apply`.
- A change takes up to approximately 15 minutes: the apply, the ESO refresh and the CronJob schedule.
- The Terraform identity of the workflow needs permission to write Entra ID groups and Key Vault secrets.

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

# Prior art
[prior-art]: #prior-art

- [tailscale/gitops-acl-action](https://github.com/tailscale/gitops-acl-action) uses the same model for Tailscale. Git wins, and the action writes a warning when an admin changed the policy in the admin console.
- [YouSysAdmin/headscale-pf](https://github.com/YouSysAdmin/headscale-pf) writes group members from an identity provider into a HuJSON policy template. This RFC does the same with `templatefile` in Terraform.

# Unresolved questions
[unresolved-questions]: #unresolved-questions

- Which Key Vault contains `headscale-acl`, and does the workflow identity have the role Key Vault Secrets Officer on it?
- After a `headscale policy set`, does `headscale policy get` return the same bytes, with the same last newline? If not, the script must remove trailing white space before `cmp`. Test this after the first run. Before that run, the live policy is always different (see [Migration](#migration)).
- Is `grpc_allow_insecure` set in the Headscale configuration, or does the CronJob need TLS for gRPC?
- How do we rotate the API key? Headscale API keys expire after 90 days by default. We must use the same procedure as for the Headplane API key.

# Future possibilities
[future-possibilities]: #future-possibilities

- Change the VPN access request template: one pull request replaces the manual steps.
- Manage `tagOwners` and `hosts` from Terraform variables.
- Add a Grafana alert for a failed CronJob run and for an overwritten edit.
