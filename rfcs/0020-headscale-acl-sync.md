- Feature Name: headscale_acl_sync
- Title: Headscale ACL policy in Git with Entra ID group sync
- Start Date: 2026-10-18
- RFC PR: [altinn/altinn-platform#4233](https://github.com/Altinn/altinn-platform/pull/4233)
- Github Issue: None.
- Product/Category: VPN (Headscale)
- State: **REVIEW** (possible states are: **REVIEW**, **ACCEPTED** and **REJECTED**)

# Summary
[summary]: #summary

This RFC adds a sync tool for the Headscale Access Control List (ACL) policy.
The full policy file `acl.hujson` is in Git, and Git shows every change.
Admins can still edit the policy in Headplane, with the Headscale API, or in Git.
The tool copies changes in both directions between Git and the Headscale database.
The tool also writes the members of the ACL groups from Microsoft Entra ID groups.
This removes the manual step "add the user to the correct ACL group(s)" from VPN access requests.

# Motivation
[motivation]: #motivation

Headscale keeps the ACL policy in one of two modes:

| Mode | Where the policy is | Edit in Headplane or API | History in Git | Apply a change |
|---|---|---|---|---|
| `database` | The Headscale database | Yes | No | Immediately, without a restart |
| `file` | A file on disk | No. Headscale rejects policy writes through the API. | Yes | Restart of Headscale |

In file mode, Headscale reads the policy file at startup.
Headscale can also reload the file on a `SIGHUP` signal.
In Kubernetes, nothing sends this signal when a ConfigMap or Secret changes.
Thus each change to the policy needs a restart of the Headscale pod.
Headscale supports only one replica, so the control plane is not available during the restart.
Clients keep their connections, but they cannot log in, register or get new peers during this time.

We want these three properties:

- Edits in Headplane and with the API.
- A full history of the policy in Git.
- A change to the policy applies without a restart of Headscale. We want to edit the policy in Headplane and with the API. We also want a full history of the policy in Git.

The current process has these problems:

- The ACL policy is not in Git. We cannot see who changed a rule, when, or why.
- We cannot review a change to the policy before it takes effect.
- If we lose the Headscale database, we lose the policy.
- Each VPN access request has a manual step to add the user to ACL groups. The [VPN access request template](../.github/ISSUE_TEMPLATE/vpn-access-request.yml) has two steps: add the user to the Entra ID group `headscale-vpn-access`, and add the user to the correct ACL group(s). The ACL group step is a second, manual copy of the group membership.

The expected outcome:

- Git has the full `acl.hujson`, with group names, group members and rules.
- Headplane, the Headscale API and Git all stay editable.
- Entra ID is the only place where admins manage group membership.
- A new Headscale database gets the policy from Git automatically.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation

## New concepts

| Concept | Meaning |
|---|---|
| Policy file | `acl.hujson` in Git. It contains the full Headscale ACL policy in HuJSON format (JSON with comments). |
| Live policy | The policy in the Headscale database. Headscale enforces this policy. |
| Sync tool | A Kubernetes CronJob. It keeps the policy file and the live policy equal. |
| Group mapping | `acl-groups.yaml` in Git. It connects an ACL group name to an Entra ID group. |
| Mapped group | An ACL group that is in the group mapping. The sync tool owns its members. |
| Manual group | An ACL group that is not in the group mapping. People own its members. |

## Example: grant VPN access to a new user

1. The approver confirms the VPN access request issue.
2. The admin adds the user to the Entra ID group `headscale-vpn-access`.
3. The admin adds the user to the Entra ID group for the team, for example `headscale-team-core`.
4. The sync tool runs. It adds the user to `group:team-core` in the live policy.
5. The sync tool commits the changed `acl.hujson` to Git.

The admin does not edit the ACL policy.

## Example: change a rule in Headplane

1. The admin opens the ACL editor in Headplane and adds a rule.
2. Headplane writes the rule to the live policy. The rule takes effect immediately.
3. The sync tool runs. It finds that the live policy changed.
4. The sync tool commits the new `acl.hujson` to Git. The commit message identifies the change as "live policy changed".

## Example: change a rule in Git

1. The admin opens a pull request that changes `acl.hujson`.
2. CI runs `headscale policy check` on the file. A reviewer approves the pull request.
3. The admin merges the pull request.
4. The sync tool runs. It finds that the policy file changed.
5. The sync tool writes the policy file to the live policy.

## Example: add a new mapped group

1. The admin creates the Entra ID group `headscale-team-tilgangsinfo`.
2. The admin opens a pull request that adds this line to `acl-groups.yaml`:

   ```yaml
   groups:
     group:team-core: headscale-team-core
     group:team-tilgangsinfo: headscale-team-tilgangsinfo
   ```

3. The same pull request adds rules for `group:team-tilgangsinfo` to `acl.hujson`.
4. The admin merges the pull request. The sync tool writes the members and the rules.

## What the policy file looks like

```hujson
{
  // Mapped groups. The sync tool writes the members from Entra ID.
  // Do not edit the members of a mapped group. The sync tool overwrites them.
  "groups": {
    "group:team-core": ["user.one@digdir.no", "user.two@digdir.no"],
    "group:team-tilgangsinfo": [],

    // Manual group. People own the members.
    "group:break-glass": ["admin.one@digdir.no"],
  },

  "acls": [
    {
      "action": "accept",
      "src": ["group:team-core"],
      "dst": ["tag:core-db:5432"],
    },
  ],
}
```

The group names and the members are in the file. A person who writes a rule can see which groups exist and who is in each group.

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

## Components

| Component | Description |
|---|---|
| Headscale | Stays in `policy.mode: database`. |
| Sync tool | A Go program in a container image. It runs as a Kubernetes CronJob every 5 minutes. |
| Sync state | A ConfigMap. It contains the SHA-256 hash of the policy at the last successful sync. |
| Policy file | `headscale-acl/acl.hujson` in [`dis-way/adminservices`](https://github.com/dis-way/adminservices). |
| Group mapping | `headscale-acl/acl-groups.yaml` in the same repository. |
| GitHub App | Gives the sync tool permission to read and write the policy file. |
| Entra ID identity | A managed identity with a federated credential for the sync tool service account. |

## Location in Git

The policy file and the group mapping are in the private repository `dis-way/adminservices`, in the directory `headscale-acl/`.

- The repository already contains the Terraform code for Headscale: the OIDC (OpenID Connect) app registration and the Entra ID group `headscale-vpn-access`.
- The Entra ID groups for the group mapping are also in Terraform in this repository. They use the same pattern as `headscale-vpn-access`. A change of group membership is thus also a reviewed change in Git.
- The directory `headscale-acl/` is outside `tf/`. The Terraform workflows filter on paths under `tf/`, so a commit from the sync tool does not start a Terraform run.

The group mapping also contains the allowed email domains:

```yaml
allowedDomains:
  - digdir.no
groups:
  group:team-core: headscale-team-core
  group:team-tilgangsinfo: headscale-team-tilgangsinfo
```

## Access to Headscale

The sync tool connects to the Headscale gRPC API.
It uses `HEADSCALE_CLI_ADDRESS` and `HEADSCALE_CLI_API_KEY`.
An ExternalSecret gets the API key from Azure Key Vault.

The sync tool uses these operations:

| Operation | Use |
|---|---|
| `headscale policy get` | Read the live policy. |
| `headscale policy check -f <file>` | Validate a policy before a write. |
| `headscale policy set -f <file>` | Write the live policy. |

## Access to Git

The sync tool uses a GitHub App installation token with `contents: write` on one repository.
The sync tool commits with the GitHub Contents API.
GitHub signs commits that a GitHub App makes through the API, so the commits show as verified.
The cluster does not need an SSH key or a signing key.
An ExternalSecret gets the GitHub App private key from Azure Key Vault.

## Access to Entra ID

The sync tool uses Azure workload identity to get a Microsoft Graph token.
The managed identity has these Microsoft Graph application permissions:

- `GroupMember.Read.All`
- `User.ReadBasic.All`

The sync tool reads the members with this request:

```
GET /v1.0/groups/{id}/transitiveMembers/microsoft.graph.user?$select=mail,userPrincipalName,userType
```

`transitiveMembers` includes the members of nested groups.
The sync tool reads all pages of the result.

## One sync run

1. Read the live policy, the policy file, the group mapping and the sync state.
2. Calculate `hash_live`, `hash_git` and `hash_last`.
3. Select the base policy with the table in [Change detection](#change-detection).
4. Get the members of each mapped group from Entra ID.
5. Replace the members of each mapped group in the base policy. Do not change other parts of the policy.
6. Run `headscale policy check` on the result.
7. If the result is different from the live policy, run `headscale policy set`.
8. If the result is different from the policy file, commit the result to Git.
9. Write the hash of the result to the sync state.

## Change detection

| Live policy changed since last sync | Policy file changed since last sync | Base policy | Result |
|---|---|---|---|
| No | No | Live policy | Only group members can change. |
| Yes | No | Live policy | The tool commits the live policy to Git. |
| No | Yes | Policy file | The tool writes the policy file to the live policy. |
| Yes | Yes | None | Conflict. The tool stops. See [Conflicts](#conflicts). |
| Live policy is empty | Any | Policy file | Bootstrap. The tool writes the policy file to the live policy. |

The hash is calculated on the policy after step 5. A change of group members in Entra ID gives a new hash after the run. The next run does not see this as a change by a person.

## Edits to HuJSON

The sync tool uses the Go library [`github.com/tailscale/hujson`](https://github.com/tailscale/hujson).
The library changes one value in a HuJSON document and keeps the comments and the format of all other values.
The sync tool changes only the member lists of mapped groups.
A library that converts HuJSON to plain JSON removes the comments. Do not use such a library.

## Rules for groups

| Case | Behaviour |
|---|---|
| Mapped group | The sync tool replaces the members on each run. |
| Manual group | The sync tool does not change the group. |
| Mapped group is empty in Entra ID | The tool writes `"group:x": []`. The group stays in the policy, so the rules that use it stay valid. |
| Mapped group is not in the policy | The tool adds the group with its members. |
| Entra ID group does not exist | The run fails. The tool does not change the live policy. |
| A user is in more than one mapped group | The tool adds the user to each group. |

## User identifier

Headscale matches an ACL user by the email that Headscale stores for the OIDC user.
For our users, this email has the format `name@digdir.no`.
The sync tool must write the same identifier.

The users are guest users (Entra ID B2B) in the tenant of the Headscale app registration.
The `userPrincipalName` of a guest user is not the email. It has this format:

```
first.last_digdir.no#EXT#@<tenant>.onmicrosoft.com
```

The sync tool normalizes each member to an identifier with these steps:

1. If `mail` is not empty, use `mail`.
2. If `mail` is empty and `userPrincipalName` contains `#EXT#`, use the part before `#EXT#`. Replace the last `_` in this part with `@`.
3. If `mail` is empty and `userPrincipalName` does not contain `#EXT#`, use `userPrincipalName`.
4. Change the identifier to lowercase.
5. If the domain of the identifier is not in `allowedDomains`, do not add the member. Write a warning to the job log.

Step 2 replaces the last `_`, because the local part of an email can contain `_`.

| `mail` | `userPrincipalName` | Identifier |
|---|---|---|
| `First.Last@digdir.no` | `first.last_digdir.no#EXT#@<tenant>.onmicrosoft.com` | `first.last@digdir.no` |
| (empty) | `first.last_digdir.no#EXT#@<tenant>.onmicrosoft.com` | `first.last@digdir.no` |
| (empty) | `first_last_digdir.no#EXT#@<tenant>.onmicrosoft.com` | `first_last@digdir.no` |
| `aas-user@ai-dev.no` | `aas-user_ai-dev.no#EXT#@<tenant>.onmicrosoft.com` | Not added. The domain is not allowed. |
| (empty) | `user@<tenant>.onmicrosoft.com` | Not added. The domain is not allowed. |

The domain filter applies to mapped groups only.
The sync tool does not remove members from manual groups.
The [CI check](#ci-check) rejects a member with a domain that is not allowed in all groups.

A user does not have to log in to Headscale before the sync tool adds the user to a group.

## Conflicts

A conflict occurs when the live policy and the policy file both change between two runs.
The tool does not merge the two policies.

1. The tool does not change the live policy or the policy file.
2. The tool opens a pull request with the live policy on a branch `acl-sync/conflict`.
3. The run fails. The CronJob failure alert tells the admins.
4. An admin resolves the conflict in Git and merges.
5. The admin deletes the sync state. The next run uses the policy file as the base policy.

## Failures

The tool must not change the live policy if it does not have all data.

| Failure | Behaviour |
|---|---|
| Microsoft Graph error, timeout or missing page | The run fails. No change. |
| `headscale policy check` fails | The run fails. No change. The error is in the job log. |
| GitHub API error after `policy set` | The run fails. The next run sees that the live policy changed, and commits it. |
| Headscale API is not available | The run fails. No change. |

## CI check

A GitHub Actions workflow runs on each pull request that changes the policy file or the group mapping.

1. Check that each group in the group mapping is in `acl.hujson`.
2. Check that each member of each group has a domain in `allowedDomains`.
3. Run `headscale policy check -f acl.hujson` with the same Headscale version as the cluster.

## Concurrency

The CronJob has `concurrencyPolicy: Forbid`. Only one run occurs at a time.

# Drawbacks
[drawbacks]: #drawbacks

- The sync tool is new code that the team must maintain.
- Changes in Entra ID and Headplane make commits from a bot in Git.
- A change in Headplane takes effect before a review. Git shows the change after it occurred.
- An admin can edit the members of a mapped group in Headplane. The sync tool removes the edit without a message to the admin.
- The sync tool needs three credentials: a Headscale API key, a GitHub App key, and an Entra ID federated identity.
- A change in Git takes effect up to 5 minutes after the merge.

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

This design is the only one that we found that gives all these properties:

- Edits in Headplane and with the API.
- A full history in Git.
- Changes apply without a restart of Headscale.
- Group membership from Entra ID.

| Alternative | Why we do not choose it |
|---|---|
| `policy.mode: file` with a ConfigMap | Headplane and the API cannot edit the policy. Each change needs a restart of Headscale. |
| Policy in Azure Key Vault, written by Terraform, read by an ExternalSecret | This is the same as file mode, with more steps. Each change needs a restart of Headscale. Edits in Headplane do not go back to Key Vault or Terraform. The policy is configuration, not a secret. |
| ExternalSecret and PushSecret for two-way sync | The live policy is in the Headscale database, not in a Kubernetes Secret. An ExternalSecret and a PushSecret on the same Secret overwrite each other. Key Vault versions are not a Git history. |
| A Job that writes the policy file to the live policy when Git changes | The next change in Git removes edits from Headplane without a message. |
| A CronJob that only exports the live policy to Git | Git is a backup only. Admins cannot change the policy in Git. |
| Policy file in Git without the `groups` section, and the tool adds groups before `policy set` | A person who writes a rule cannot see the group names and members in Git. |
| Group mapping from a name prefix in Entra ID | A new Entra ID group can appear in the ACL policy without a review in Git. |

## CronJob or operator

An alternative to the CronJob is an operator with a custom resource for the group mapping.

### Sources of change

An operator reacts to changes in Kubernetes objects. Most sources of change in this design are outside Kubernetes.

| Source of change | Can an operator watch it? |
|---|---|
| Group membership in Entra ID | No. Microsoft Graph change notifications need a public webhook endpoint and subscriptions that the operator must renew. |
| Live policy in Headscale | No. Headscale does not send change events. |
| `acl.hujson` in GitHub | Only with a GitHub webhook. |
| Group mapping custom resource | Yes. |

Thus the operator must also poll every 5 minutes (`RequeueAfter: 5m`). Only a change to the group mapping starts a reconcile immediately.

### Benefits of an operator

| Benefit | Value in this design |
|---|---|
| Status on the custom resource: last sync, conflict and warnings | Medium. The job log and an alert on a failed run give most of this information. |
| A change to the group mapping applies immediately | Low. The group mapping changes seldom. 5 minutes is sufficient. |
| Validation of the group mapping in the API server (CRD schema and CEL rules) | Low. The CI check validates `acl-groups.yaml` before the merge. |
| Prometheus metrics from controller-runtime | Low. Alerts on failed jobs from kube-state-metrics are sufficient. |
| Teams own a group mapping custom resource in their namespace | High, but only for a self-service model. Today, admins approve and grant VPN access. |
| No new pod every 5 minutes | Low. This is 288 short pods each day. |

An operator also has a disadvantage. The custom resource must be in the Flux manifests for Headscale, and not in `dis-way/adminservices`. Thus `acl.hujson` and the group mapping are in two different locations.

### Estimate of work

The estimates are in person-days for one developer who knows Go. The two alternatives use the same sync core.

| Work item | CronJob | Operator |
|---|---|---|
| Sync core: hashes, change detection, HuJSON edits, conflicts | 4 | 4 |
| Microsoft Graph client, normalization of guest users, domain filter | 2 | 2 |
| Headscale gRPC client | 1 | 1 |
| GitHub App commits and conflict pull request | 2 | 2 |
| CI check workflow | 1 | 1 |
| Unit tests for the sync core | 3 | 3 |
| CRD API types, CEL validation and status conditions | 0 | 2 |
| Reconciler: requeue, status updates and backoff | 0 | 2–3 |
| Integration tests with envtest | 0 | 2–3 |
| Packaging: image, manifests, RBAC, ExternalSecrets and Terraform | 2 | 3 |
| **Total for the first version** | **15** | **22–24** |

| Maintenance item for each year | CronJob | Operator |
|---|---|---|
| Dependency updates: Microsoft Graph SDK, GitHub client, Renovate pull requests | 1–2 | 1–2 |
| Changes in the Headscale gRPC API. Headscale is version 0.x and can change the API in a minor release. | 2–3 | 2–3 |
| Updates of controller-runtime and Kubernetes libraries, and CRD version upgrades | 0 | 3–5 |
| Troubleshooting | 1 | 1–2 |
| **Total for each year** | **4–6** | **7–12** |

The platform team already builds operators with kubebuilder, for example `dis-apim-operator`, `dis-messaging-operator` and `dis-cache-operator`.
The existing scaffolding and release pipelines decrease the first version of an operator by approximately 2–3 days.
The updates of controller-runtime then occur together with the updates of the other operators.
The largest maintenance risk is the Headscale gRPC API. This risk is the same for the two alternatives.

### Decision

We use a CronJob.

- The sync core is a Go package with one function, for example `sync.Run(ctx, cfg) (Result, error)`.
- The CronJob calls this function one time for each run.
- A later reconciler can call the same function and write `Result` to the status of a custom resource.

We change to an operator only when we have a specific need, for example a self-service group mapping for teams.
An intermediate step is to run the same program as a Deployment with a loop every 5 minutes. This gives a `/metrics` endpoint and no new pod for each run, without a CRD.

If we do not do this, the ACL policy stays outside Git, and admins continue to copy group membership by hand for each VPN access request.

# Prior art
[prior-art]: #prior-art

- [tailscale/gitops-acl-action](https://github.com/tailscale/gitops-acl-action) writes a policy file from Git to Tailscale. It is one-way. An edit in the admin console is lost at the next run of the action.
- Tailscale can get users and groups from Entra ID with SCIM (System for Cross-domain Identity Management). Headscale does not support SCIM.
- Headscale supports `oidc.allowed_groups`. It controls which Entra ID groups can log in. It does not write ACL groups.
- Flux and Argo CD use Git as the only source of truth and overwrite changes in the cluster. This RFC does not do that, because we want edits in Headplane to stay.

# Unresolved questions
[unresolved-questions]: #unresolved-questions

- Must the sync tool commit directly to the default branch, or open a pull request for each change from the live policy?
- The ruleset `protect-main` in `dis-way/adminservices` requires a pull request, but it is disabled now. If we enable it, the GitHub App must be a bypass actor, or the sync tool must open pull requests.
- Which Entra ID groups and ACL group names do we start with?
- Where must the source code of the sync tool be, and which GitHub App must it use?
- Is 5 minutes the correct interval?

# Future possibilities
[future-possibilities]: #future-possibilities

- Set `oidc.allowed_groups` to the union of the mapped Entra ID groups. Only users in a mapped group can log in.
- Change the VPN access request template to remove the manual ACL group step.
- Sync `tagOwners` from Entra ID groups with the same group mapping.
- Replace the CronJob with an operator and a custom resource for the group mapping, if teams need a self-service group mapping. See [CronJob or operator](#cronjob-or-operator).
- Add a Grafana alert for a failed sync run and for a conflict.
