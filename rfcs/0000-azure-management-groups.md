- Feature Name: azure_management_groups
- Start Date: 2024-09-06
- RFC PR: [altinn/altinn-platform#0000](https://github.com/Altinn/altinn-platform/pull/916)
- Github Issue: [altinn/altinn-platform#861](https://github.com/Altinn/altinn-platform/issues/914)
- Product/Category: (optional)
- State: **REVIEW** (possible states are: **REVIEW**, **ACCEPTED** and **REJECTED**)

# Summary
[summary]: #summary

This RFC proposes a solution for structuring management groups in Azure for Altinn. Management Groups are highly useful for structuring and managing resources across subscriptions, teams, products, and environments. A well-organized structure establishes the foundation for maintaining Identity and Access Management (IAM) and Azure Policies consistently across all teams within Altinn.

# Motivation
[motivation]: #motivation

There are several key reasons why structuring Altinn resources into Azure Management Groups is essential:

There's been occasions where teams requiring access to Azure resources creates a ticket for the platform team. This workflow involves detailing who needs access to which resources, even though this information is often already well-known within the requesting team. The platform team, however, doesn't need such specific details and shouldn't be burdened with managing access for every individual team. This leads to several inefficiencies:

* Delays in Access: Teams must wait for the platform team to prioritize and process the ticket, which can delay their progress.
* Architects' Time: Technical architects, who are already occupied with higher-priority tasks, are forced to spend time writing these tickets. Managing IAM for Azure resources is something architects should have the authority to handle within their own teams, without needing to involve the platform team.
* Unnecessary Overhead and Costs: Every access request consumes time in platform meetings, with resources spent discussing and prioritizing minor tasks. This results in direct and opportunity costs, as teams are slowed down and platform resources are inefficiently used.

Implementing a hierarchy of Azure Management Groups offers significant advantages in terms of cost management and accountability. This structure enables efficient tracking of cloud spending and budgeting across different teams within Altinn. While cost management is not typically a top priority for technical teams, automating certain tasks—such as cost tracking and reporting—can provide valuable insights for the finance department, helping to streamline financial operations and ensure budgetary control.

A management group structure that mirrors our internal communication patterns will provide a uniform picture across the organization, ensuring consistency and clarity across teams. This approach streamlines processes, enhances template usage, and fosters improved collaboration between teams. By empowering each team to work within familiar guardrails, we create an environment where individuals can operate more effectively. Embracing such a structure simplifies governance and communication, allowing teams to seamlessly collaborate across departments with a well-known and cohesive setup.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation