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

Explain the proposal as if it was already included in the platform and you were teaching it to another member of the team. That generally means:

- Introducing new named concepts.
- Explaining the feature largely in terms of examples.
- Explaining how team members should *think* about the feature, and how it should impact the way they use the platform product. It should explain the impact as concretely as possible.
- If applicable, provide sample error messages, deprecation warnings, or migration guidance.
- If applicable, describe the differences between teaching this to existing team members and new team members.
- If applicable, discuss how this impacts the ability to read, understand, and maintain sofware that runs in or uses the platform. Code is read and modified far more often than written; will the proposed feature make services easier to maintain? Will this help users of the platform to maintain their services?

For implementation-oriented RFCs (e.g. for a code based solution), this section should focus on how code contributors should think about the change, and give examples of its concrete impact. For policy RFCs, this section should provide an example-driven introduction to the policy, and explain its impact in concrete terms.

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
    js ->> jq: Check for Pending Job
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

Why should we *not* do this?

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

- Why is this design the best in the space of possible designs?
- What other designs have been considered and what is the rationale for not choosing them?
- What is the impact of not doing this?

# Prior art
[prior-art]: #prior-art

Discuss prior art, both the good and the bad, in relation to this proposal.
A few examples of what this can include are:

- For any of the platform products: Does this feature exist in other projects and what experience have their community had?
- For community proposals: Is this done by some other community and what were their experiences with it?
- For other teams: What lessons can we learn from what other communities have done here?


This section is intended to encourage you as an author to think about the lessons from other projects, provide readers of your RFC with a fuller picture.
If there is no prior art, that is fine - your ideas are interesting to us whether they are brand new or if it is an adaptation from other languages.

Note that while precedent set by other projects is some motivation, it does not on its own motivate an RFC.

# Unresolved questions
[unresolved-questions]: #unresolved-questions

- What parts of the design do you expect to resolve through the RFC process before this gets merged?
- What parts of the design do you expect to resolve through the implementation of this feature before stabilization?
- What related issues do you consider out of scope for this RFC that could be addressed in the future independently of the solution that comes out of this RFC?
- What are the (unknown) unknowns?

# Future possibilities
[future-possibilities]: #future-possibilities

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