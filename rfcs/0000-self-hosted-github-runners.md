- Feature Name: github_self_hosted_runner
- Start Date: 2024-08-28
- RFC PR: [altinn/altinn-platform#0000](https://github.com/altinn/altinn-platform/pull/0000)
- Github Issue: [altinn/altinn-platform#861](https://github.com/Altinn/altinn-platform/issues/861)
- Product/Category: (optional)
- State: **REVIEW** (possible states are: **REVIEW**, **ACCEPTED** and **REJECTED**)

# Summary
[summary]: #summary

The proposed RFC introduces self-hosted runners in GitHub that teams can set up using a straightforward self-service solution. Self-hosted runners are useful in scenarios where exceptions in firewalls are needed or in private projects that consume all the free minutes available for GitHub-hosted runners. The runners should be discarded after each job to reduce the chances of contamination. The number of runners should scale up and down automatically.

# Motivation
[motivation]: #motivation

We have some use cases where we need to run pipelines that access internal resources within our infrastructure. We cannot allow access to GitHub-hosted runners, as this would remove a layer of security in our solution. By setting up self-hosted runners, we can place these in a VNet we control and enable secure communication between those VNets and other private VNets.

Each product/team should have its own set of self-hosted runners located in a VNet, so we don't need to expose all services to all runners. Self-hosted runners in a public repository introduce a security concern, as jobs can be executed from forks. To reduce the risk, a runner should only run one job and then be discarded (ephemeral runners). Additionally, all services or infrastructure in the VNets connected to the runner VNet should be protected by a secure login/authentication barrier, ensuring that access to the runner is not enough to access private services.

Teams have different needs, and our solution should support these varying needs without adding too much of a maintenance burden on the platform team. The number of runners should scale up if there are queued jobs and down to zero if there are none. This ensures that we support both use cases with a high number of executions and those with few, without paying for unused resources.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation

To make it easy for teams in need of self-hosted runners the platform team provides a base oci image for github runners and a terraform module for setting up the azure container apps in a subscription and vnet they control.

The base oci image is published on ghcr.io and also available in our pull through cache of images in azure.
The terraform module is published through our provider in the terraform registry.

Managing the resources through publicly available registries like ghcr.io and registry.terraform.io will simplify the process of keeping dependencies up to date for teams leveraging our buildingblocks with services like dependabot or renovate.

The teams will need to contact team platform to get access to the gituhb apps private key so they can have access to register their own runners with the apps credentials. We should keep the use of PAT to a minimum.

<!--
Explain the proposal as if it was already included in the platform and you were teaching it to another member of the team. That generally means:

- Introducing new named concepts.
- Explaining the feature largely in terms of examples.
- Explaining how team members should *think* about the feature, and how it should impact the way they use the platform product. It should explain the impact as concretely as possible.
- If applicable, provide sample error messages, deprecation warnings, or migration guidance.
- If applicable, describe the differences between teaching this to existing team members and new team members.
- If applicable, discuss how this impacts the ability to read, understand, and maintain sofware that runs in or uses the platform. Code is read and modified far more often than written; will the proposed feature make services easier to maintain? Will this help users of the platform to maintain their services?

For implementation-oriented RFCs (e.g. for a code based solution), this section should focus on how code contributors should think about the change, and give examples of its concrete impact. For policy RFCs, this section should provide an example-driven introduction to the policy, and explain its impact in concrete terms.
-->

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

## Infrastructure overview

```mermaid
C4Context
title System Context Diagram for self-hosted runners

Deployment_Node(gh, "GitHub", "") {
    Deployment_Node(altinn, "Organization", "Altinn") {
        SystemDb(repo1, "Git Repo", "github.com/altinn/altinn-platform")
        SystemQueue(jobq1, "Workflow Job Queue")
    }
}
Deployment_Node(az, "Azure", "") {
    Deployment_Node(acavnet, "VNet", "Runner VNet") {
        Deployment_Node(avaenv, "Azure Container App Environment", "Product/Team Environment") {
            Container_Boundary(job, "Azure Container App Job") {
                Container(acascaler1, "Job Scaler")
                Container(aca1, "Azure Container App Job")
            }
        }
    }

    Boundary(svc, "Private VNet") {
        SystemDb(db, "PostgreSQL")
    }
}

Rel(repo1, jobq1, "Queue Job")
Rel(acascaler1, jobq1, "Checks for Queued Jobs")
Rel(acascaler1, aca1, "Starts Job")
Rel(aca1, repo1, "Register Runner")
Rel(aca1, db, "Connect to Database")
```
## Lifecycle of Containers Running Jobs

```mermaid
sequenceDiagram
participant w as Workflows
participant jq as Job Queue
participant js as Custom Scale Rule

loop every 20 seconds
    js ->> jq: Check for Pending Job
end
par
    w ->> jq: Queue Job
    js --> jq: Pending job is discovered
    create participant aj1 as Azure Container App Job 1
    js ->> aj1: Start Azure Container App Job
    aj1 ->> w: Register Runner with Repo
    w ->> aj1: Send Job to Runner
    loop foreach step in job
        aj1 ->> aj1: Execute Step
    end
    aj1 ->> w: Send Back Result of Job
    destroy aj1
    aj1 -x w: Deregister and Terminate
end
par
    w ->> jq: Queue Job
    js --> jq: Pending job is discovered
    create participant aj2 as Azure Container App Job 2
    js ->> aj2: Start Azure Container App Job
    aj2 ->> w: Register Runner with Repo
    w ->> aj2: Send Job to Runner
    loop foreach step in job
        aj2 ->> aj2: Execute Step
    end
    aj2 ->> w: Send Back Result of Job
    destroy aj2
    aj2 -x w: Deregister and Terminate
end
```

This is the technical portion of the RFC. Explain the design in sufficient detail that:

- Its interaction with other features is clear.
- It is reasonably clear how the feature would be implemented.
- Corner cases are dissected by example.

The section should return to the examples given in the previous section, and explain more fully how the detailed proposal makes those examples work.

# Drawbacks
[drawbacks]: #drawbacks

- Self-hosted runners are a security risk as actors can create PRs from forks and run any code they want on our infrastructure.
- Self-hosted runners can expose weaknesses if hosts in private networks assume they are safe because they are not exposed publicly. This should not be the case, but private networking tends to lead people to make that assumption. A zero-trust mindset is a must.

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

- Alternatives include setting up VMs to run jobs from workflows; this removes the possibility of having ephemeral runners and autoscaling, which introduces even greater security risks.
- Using an AKS cluster with an operator for creating and handling pods for running GitHub Action jobs on self-hosted runners. This is less of a security risk than VMs since we can have ephemeral runners, but there is still a risk if you escape the Kubernetes pod sandbox, as the host running the AKS node is still present.
- Upgrading our GitHub plan to use [private networking for GitHub-hosted runners in your organization](https://docs.github.com/en/organizations/managing-organization-settings/about-azure-private-networking-for-github-hosted-runners-in-your-organization).

The alternative requiring less effort on our end is paying for GitHub Team and setting up private networking. We would still need to provide some way for teams to configure this. 

Azure Container Apps are also, of course, running on some sort of hardware/host somewhere, but since that sandbox is managed by Azure, it is safe to assume that more time, effort, and funding go into keeping it safe than we could ever invest in our VMs/AKS nodes. The added benefit of not having to pay for compute resources when we are not using them makes it a good second alternative to paying for GitHub Team.

# Prior art
[prior-art]: #prior-art

We already have some private runners for some private repositories. These run in a Kubernetes cluster with an operator. This works, but there is still higher maintenance since we have to ensure that Kubernetes, the host OS, and other software/hardware are kept up to date. If we change these to Azure Container Apps, we should only need to keep the Docker image up to date.

<!-- 
Discuss prior art, both the good and the bad, in relation to this proposal.
A few examples of what this can include are:

- For any of the platform products: Does this feature exist in other projects and what experience have their community had?
- For community proposals: Is this done by some other community and what were their experiences with it?
- For other teams: What lessons can we learn from what other communities have done here?


This section is intended to encourage you as an author to think about the lessons from other projects, provide readers of your RFC with a fuller picture.
If there is no prior art, that is fine - your ideas are interesting to us whether they are brand new or if it is an adaptation from other languages.

Note that while precedent set by other projects is some motivation, it does not on its own motivate an RFC.
-->

# Unresolved questions
[unresolved-questions]: #unresolved-questions

- How do we make it easy to configure connections between a team's GitHub runners VNet and their internal VNets?
- How can we discover if a team using a custom image has a security vulnerability and needs to upgrade it?
- How do we monitor the runners for suspicious traffic/processes?

<!--
- What parts of the design do you expect to resolve through the RFC process before this gets merged?
- What parts of the design do you expect to resolve through the implementation of this feature before stabilization?
- What related issues do you consider out of scope for this RFC that could be addressed in the future independently of the solution that comes out of this RFC?
- What are the (unknown) unknowns?
 -->
# Future possibilities
[future-possibilities]: #future-possibilities

As a first extension of this we should make the teams able to setup a self-hosted runner environment without requesting a key from platform, but for now we can make this a manual initial setup to get some experience with self hosted runners

Once we see the usage and how the runners are used the platform team can make informed decisions on how we can evolve this service so that the teams might not need to run and maintain their own infrastructure related to the self-hosted runners and just use a fully managed service by the platform team, but for now we make a MVP that solves the use case we currently have.

<!--
Think about what the natural extension and evolution of your proposal would
be and how it would affect the project as a whole in a holistic
way. Try to use this section as a tool to more fully consider all possible
interactions with the project in your proposal.
Also consider how this all fits into the roadmap for the project
and of the relevant sub-team.

This is also a good place to "dump ideas", if they are out of scope for the
RFC you are writing but otherwise related.

If you have tried and cannot think of any future possibilities,
you may simply state that you cannot think of anything.

Note that having something written down in the future-possibilities section
is not a reason to accept the current or a future RFC; such notes should be
in the section on motivation or rationale in this or subsequent RFCs.
The section merely provides additional information.
-->
