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

| Mode | Edit in Headplane or API | History in Git | Apply a change |
|---|---|---|---|
| `database` | Yes | No | Immediately |
| `file` | No. Headscale rejects policy writes through the API. | Yes | Restart of Headscale |

In file mode, Headscale reads the policy file at startup or on a `SIGHUP` signal.
In Kubernetes, nothing sends this signal when a ConfigMap or Secret changes, so each change needs a restart.
Headscale supports only one replica. During the restart, clients cannot log in, register or get new peers.

The current process has these problems:

- The ACL policy is not in Git. We cannot see who changed a rule, when, or why.
- If we lose the Headscale database, we lose the policy.
- Each [VPN access request](../.github/ISSUE_TEMPLATE/vpn-access-request.yml) has a manual step to add the user to ACL groups. This step is a second, manual copy of the Entra ID group membership.

The expected outcome:

- Git has the full `acl.hujson`, with group names, group members and rules.
- Headplane, the Headscale API and Git all stay editable.
- A change applies without a restart of Headscale.
- Entra ID is the only place where admins manage group membership.
- A new Headscale database gets the policy from Git automatically.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation

| Concept | Meaning |
|---|---|
| Policy file | `acl.hujson` in Git. The full Headscale ACL policy in HuJSON format (JSON with comments). |
| Live policy | The policy in the Headscale database. Headscale enforces this policy. |
| Sync tool | A Kubernetes CronJob. It keeps the policy file and the live policy equal. |
| Group mapping | `acl-groups.yaml` in Git. It connects an ACL group name to an Entra ID group. |
| Mapped group | An ACL group in the group mapping. The sync tool owns its members. |
| Manual group | An ACL group that is not in the group mapping. People own its members. |

**Grant VPN access to a new user:**

1. The admin adds the user to the Entra ID groups `headscale-vpn-access` and `headscale-team-core`.
2. The sync tool adds the user to `group:team-core` in the live policy.
3. The sync tool commits the changed `acl.hujson` to Git.

The admin does not edit the ACL policy.

**Change a rule:**

- In Headplane or with the API: the rule takes effect immediately. The sync tool commits the change to Git at the next run.
- In Git: CI runs `headscale policy check` on the pull request. After the merge, the sync tool writes the rule to the live policy at the next run.

**The policy file:**

```hujson
{
  // Mapped groups. The sync tool writes the members from Entra ID.
  // Do not edit the members of a mapped group. The sync tool overwrites them.
  "groups": {
    "group:team-core": ["user.one@digdir.no", "user.two@digdir.no"],

    // Manual group. People own the members.
    "group:break-glass": ["admin.one@digdir.no"],
  },

  "acls": [
    {"action": "accept", "src": ["group:team-core"], "dst": ["tag:core-db:5432"]},
  ],
}
```

A person who writes a rule can see which groups exist and who is in each group.

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

## Components

| Component | Description |
|---|---|
| Headscale | Stays in `policy.mode: database`. |
| Sync tool | A Go program. It runs as a Kubernetes CronJob every 5 minutes with `concurrencyPolicy: Forbid`. |
| Sync state | A ConfigMap with the SHA-256 hash of the policy at the last sync. |
| Policy file and group mapping | `headscale-acl/acl.hujson` and `headscale-acl/acl-groups.yaml` in [`dis-way/adminservices`](https://github.com/dis-way/adminservices). |

`dis-way/adminservices` already contains the Terraform code for the Headscale OIDC (OpenID Connect) app registration and the Entra ID group `headscale-vpn-access`.
The mapped Entra ID groups are also in Terraform there, so a change of group membership is also a reviewed change in Git.
The directory `headscale-acl/` is outside `tf/`, so a commit from the sync tool does not start a Terraform workflow.

```yaml
# acl-groups.yaml
allowedDomains:
  - digdir.no
groups:
  group:team-core: headscale-team-core
```

## Access

| System | Credential | Use |
|---|---|---|
| Headscale gRPC API | API key from Azure Key Vault, through an ExternalSecret | `headscale policy get`, `policy check` and `policy set` |
| GitHub | GitHub App with `contents: write`. Private key from Azure Key Vault. | Commits through the Contents API. GitHub signs these commits, so the cluster does not need a signing key. |
| Microsoft Graph | Azure workload identity. Application permissions `GroupMember.Read.All` and `User.ReadBasic.All`. | `GET /v1.0/groups/{id}/transitiveMembers/microsoft.graph.user?$select=mail,userPrincipalName`. The tool reads all pages. |

## One sync run

1. Read the live policy, the policy file, the group mapping and the sync state.
2. Select the base policy with the table below.
3. Get the members of each mapped group from Entra ID.
4. Replace the members of each mapped group in the base policy. Do not change other parts of the policy.
5. Run `headscale policy check` on the result.
6. If the result is different from the live policy, run `headscale policy set`.
7. Write the hash of the result to the sync state.
8. If the result is different from the policy file, commit the result to Git.

The tool writes the sync state before the commit. If the commit fails, the next run sees only a change in Git, uses the policy file as the base, and makes the commit again.

| Live policy changed since last sync | Policy file changed since last sync | Base policy | Result |
|---|---|---|---|
| No | No | Live policy | Only group members can change. |
| Yes | No | Live policy | The tool commits the live policy to Git. |
| No | Yes | Policy file | The tool writes the policy file to the live policy. |
| Yes | Yes | None | Conflict. See below. |
| Live policy is empty | Any | Policy file | Bootstrap. |

**Conflict:** the tool does not merge.
It opens a pull request with the live policy on the branch `acl-sync/conflict`, and the run fails.
An admin resolves the conflict in Git, merges, and deletes the sync state.
Without a sync state, the tool uses the policy file as the base.

**Failures:** the tool must not change the live policy if it does not have all data.
A Microsoft Graph error, a missing page, a failed `policy check` or an unavailable Headscale API stops the run without a change.

## HuJSON edits

The sync tool uses [`github.com/tailscale/hujson`](https://github.com/tailscale/hujson).
The library changes one value and keeps the comments and format of the other values.
Do not convert the policy to plain JSON, because this removes the comments.

## Rules for groups

| Case | Behaviour |
|---|---|
| Mapped group | The tool replaces the members. If a person changed the members since the last sync, the tool writes a warning with the changes to the job log. |
| Manual group | The tool does not change the group. |
| Mapped group is empty in Entra ID | The tool writes `"group:x": []`, so the rules that use it stay valid. |
| Entra ID group does not exist | The run fails without a change. |

## User identifier

Headscale stores OIDC users as `name@digdir.no`. The sync tool must write the same identifier.
The users are guest users (Entra ID B2B). The `userPrincipalName` of a guest user has the format `first.last_digdir.no#EXT#@<tenant>.onmicrosoft.com`.

1. If `mail` is not empty, use `mail`.
2. Else, if `userPrincipalName` contains `#EXT#`, use the part before `#EXT#`. Replace the last `_` with `@`, because the local part can contain `_`.
3. Else, use `userPrincipalName`.
4. Change the identifier to lowercase.
5. If the domain is not in `allowedDomains`, do not add the member. Write a warning to the job log.

| `mail` | `userPrincipalName` | Identifier |
|---|---|---|
| (empty) | `first_last_digdir.no#EXT#@<tenant>.onmicrosoft.com` | `first_last@digdir.no` |
| `aas-user@ai-dev.no` | `aas-user_ai-dev.no#EXT#@<tenant>.onmicrosoft.com` | Not added |

## CI check

A GitHub Actions workflow runs on each pull request that changes `headscale-acl/`:

1. Each group in the group mapping is in `acl.hujson`.
2. Each member of each group has a domain in `allowedDomains`.
3. `headscale policy check -f acl.hujson` passes with the Headscale version of the cluster.

# Drawbacks
[drawbacks]: #drawbacks

- The sync tool is new code that the team must maintain.
- Changes in Entra ID and Headplane make bot commits in Git.
- A change in Headplane takes effect before a review.
- Headplane has a structured editor for groups ([tale/headplane#608](https://github.com/tale/headplane/pull/608)). An admin can edit a mapped group by mistake. The tool removes the edit, and Headplane does not show a message.
- If a save in Headplane changes the format or removes comments, each save gives a bot commit.
- A change in Git takes effect up to 5 minutes after the merge.

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

This design is the only one that we found that gives all the outcomes in [Motivation](#motivation).
If we do not do this, the ACL policy stays outside Git, and admins copy group membership by hand.

| Alternative | Why we do not choose it |
|---|---|
| `policy.mode: file` with a ConfigMap | No edits in Headplane or with the API. Each change needs a restart. |
| Policy in Azure Key Vault from Terraform, read by an ExternalSecret | The same as file mode, with more steps. The policy is configuration, not a secret. |
| ExternalSecret and PushSecret for two-way sync | The live policy is not in a Kubernetes Secret. The two resources overwrite each other. |
| A Job that writes Git to the live policy | Removes edits from Headplane without a message. |
| A CronJob that only exports the live policy to Git | Git is a backup only. |
| Policy file without the `groups` section | A person who writes a rule cannot see the groups. |
| OIDC groups in Headscale ([juanfont/headscale#3216](https://github.com/juanfont/headscale/pull/3216)) | Not merged. Groups update only at login, so a removed user keeps access until the next login. |

**CronJob or operator.**
An operator gives a status on a custom resource, but most sources of change are outside Kubernetes: Entra ID, Headscale and GitHub.
Thus an operator must also poll every 5 minutes.
The custom resource must also be in the Flux manifests for Headscale, apart from `acl.hujson`.

| Estimate (person-days) | CronJob | Operator |
|---|---|---|
| First version | 15 | 22–24 |
| Maintenance for each year | 4–6 | 7–12 |

The sync core is a Go package with one function, `sync.Run(ctx, cfg) (Result, error)`.
A later operator can call the same function. We change to an operator only for a specific need, for example a self-service group mapping for teams.

# Prior art
[prior-art]: #prior-art

We did not find a tool that syncs the Headscale ACL policy in two directions. These tools do parts of the design:

| Project | Relation to this RFC |
|---|---|
| [tailscale/gitops-acl-action](https://github.com/tailscale/gitops-acl-action) | One-way from Git to Tailscale. It detects edits in the admin console with a cached ETag. This is the same idea as the sync state. |
| [YouSysAdmin/headscale-pf](https://github.com/YouSysAdmin/headscale-pf) | Writes group members from JumpCloud, Authentik, LDAP or Keycloak into a HuJSON template and keeps the comments. No Entra ID source. A candidate for reuse: an Entra ID source upstream saves approximately 2 days. |
| [infradohq/headscale-operator](https://github.com/infradohq/headscale-operator) | Writes a policy from a custom resource with gRPC `SetPolicy`. One-way. Its `pkg/headscale` gRPC client is a candidate for reuse. |

Tailscale gets groups from Entra ID with SCIM (System for Cross-domain Identity Management). Headscale does not support SCIM.

# Unresolved questions
[unresolved-questions]: #unresolved-questions

- Must the sync tool commit directly to `main`, or open pull requests? The ruleset `protect-main` in `dis-way/adminservices` is disabled now. If we enable it, the GitHub App must be a bypass actor.
- Which Entra ID groups and ACL group names do we start with?
- Where is the source code of the sync tool, and which GitHub App does it use?
- Do we contribute an Entra ID source to headscale-pf, or write the code in the sync tool?
- Does a save in our version of Headplane keep the comments and the format of the policy?

# Future possibilities
[future-possibilities]: #future-possibilities

- Change the VPN access request template to remove the manual ACL group step.
- Set `oidc.allowed_groups` to the union of the mapped Entra ID groups.
- Sync `tagOwners` from Entra ID groups.
- If juanfont/headscale#3216 is merged: OIDC groups for access at login, and the sync tool for fast removal.
- Add a Grafana alert for a failed sync run and for a conflict.
