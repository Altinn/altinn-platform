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

| Mode | Where the policy is | Edit in Headplane or API | History in Git |
|---|---|---|---|
| `database` | The Headscale database | Yes | No |
| `file` | A file on disk | No. Headscale rejects policy writes through the API. | Yes |

We want both properties. We want to edit the policy in Headplane and with the API. We also want a full history of the policy in Git.

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
| Policy file | `acl.hujson` in a Git repository. |
| Group mapping | `acl-groups.yaml` in the same directory as the policy file. |
| GitHub App | Gives the sync tool permission to read and write the policy file. |
| Entra ID identity | A managed identity with a federated credential for the sync tool service account. |

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
GET /v1.0/groups/{id}/transitiveMembers/microsoft.graph.user?$select=mail,userPrincipalName
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

Headscale matches an ACL user by the identifier that Headscale stores for the OIDC user.
The sync tool must write the same identifier.
The default is the `mail` attribute in lowercase.
If `mail` is empty, the tool uses `userPrincipalName` and writes a warning.

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
2. Run `headscale policy check -f acl.hujson` with the same Headscale version as the cluster.

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

This design is the only one that we found that gives all three properties: edits in Headplane and the API, full history in Git, and group membership from Entra ID.

| Alternative | Why we do not choose it |
|---|---|
| `policy.mode: file` with a ConfigMap | Headplane and the API cannot edit the policy. |
| Policy in Azure Key Vault, written by Terraform, read by an ExternalSecret | This is the same as file mode, with more steps. Edits in Headplane do not go back to Key Vault or Terraform. The policy is configuration, not a secret. |
| ExternalSecret and PushSecret for two-way sync | The live policy is in the Headscale database, not in a Kubernetes Secret. An ExternalSecret and a PushSecret on the same Secret overwrite each other. Key Vault versions are not a Git history. |
| A Job that writes the policy file to the live policy when Git changes | The next change in Git removes edits from Headplane without a message. |
| A CronJob that only exports the live policy to Git | Git is a backup only. Admins cannot change the policy in Git. |
| Policy file in Git without the `groups` section, and the tool adds groups before `policy set` | A person who writes a rule cannot see the group names and members in Git. |
| Group mapping from a name prefix in Entra ID | A new Entra ID group can appear in the ACL policy without a review in Git. |

If we do not do this, the ACL policy stays outside Git, and admins continue to copy group membership by hand for each VPN access request.

# Prior art
[prior-art]: #prior-art

- [tailscale/gitops-acl-action](https://github.com/tailscale/gitops-acl-action) writes a policy file from Git to Tailscale. It is one-way. An edit in the admin console is lost at the next run of the action.
- Tailscale can get users and groups from Entra ID with SCIM (System for Cross-domain Identity Management). Headscale does not support SCIM.
- Headscale supports `oidc.allowed_groups`. It controls which Entra ID groups can log in. It does not write ACL groups.
- Flux and Argo CD use Git as the only source of truth and overwrite changes in the cluster. This RFC does not do that, because we want edits in Headplane to stay.

# Unresolved questions
[unresolved-questions]: #unresolved-questions

- Which Git repository and directory must contain `acl.hujson` and `acl-groups.yaml`?
- Must the sync tool commit directly to the default branch, or open a pull request for each change from the live policy?
- Which identifier does Headscale store for our OIDC users: `mail`, `userPrincipalName` or the username? We must check `headscale users list -o json` before the implementation.
- How must the tool handle guest users, where `userPrincipalName` contains `#EXT#`?
- Which Entra ID groups and ACL group names do we start with?
- Where must the source code of the sync tool be, and which GitHub App must it use?
- Is 5 minutes the correct interval?

# Future possibilities
[future-possibilities]: #future-possibilities

- Set `oidc.allowed_groups` to the union of the mapped Entra ID groups. Only users in a mapped group can log in.
- Change the VPN access request template to remove the manual ACL group step.
- Sync `tagOwners` from Entra ID groups with the same group mapping.
- Replace the CronJob with an operator and a custom resource for the group mapping.
- Add a Grafana alert for a failed sync run and for a conflict.
