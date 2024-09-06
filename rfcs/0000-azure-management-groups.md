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

The proposed structure of Azure Management Groups and Azure AD Groups is designed to reflect Altinn's internal organizational structure. The terminology used for the management groups should align with the language and communication patterns already familiar within Altinn. `Altinn Products` are managed by product teams that serve external customers, or service owners, and are responsible for billing. These teams focus on delivering products and services to clients. On the other hand, `Altinn Capabilities` consist of internal teams that do not necessarily sell products but instead contribute by providing insights, maintaining the Altinn platform, and developing internal applications. The terminology and grouping mirror how these teams operate and communicate within Altinn, ensuring that the structure is intuitive and aligns with the way the organization functions.

**__NOTE:__** `Capabilties` is not well-known (yet). Probably should pick a better name

```
# Management Groups
↓ Tenant Root Group
    ↓ Service-Owners
        ↓ Service-Owner-{Name}-Test
            🔑 Service-Owner-{Name}-Test
        ↓ Service-Owner-{Name}-Prod
            🔑 Service-Owner-{Name}-Test

    ↓ Altinn-Capabilities
        ↓ Altinn-{Name}-Dev
            🔑 Altinn-Capability-{Name}-Dev
            🔑 Altinn-Capability-{Name}-Test
        ↓ Altinn-{Name}-Prod
            🔑 Altinn-Capability-{Name}-Staging
            🔑 Altinn-Capability-{Name}-Prod
        ...

    ↓ Altinn-Products
        ↓ Altinn-{Name}-Dev
            🔑 Altinn-Product-{Name}-Dev
            🔑 Altinn-Product-{Name}-Test
        ↓ Altinn-{Name}-Prod
            🔑 Altinn-Product-{Name}-Staging
            🔑 Altinn-Product-{Name}-Prod
        ...

```

To manage access to these Management Groups and their connected subscriptions, a set of Azure AD Groups are assigned specific roles for each Management Group. These Azure AD groups are dynamic as it contains a set of individuals which are managed by selected administrators. The administrators of these groups shouldn't need to set direct access to any azure resources, they can administrate access to resources by adding and removing members in an AD group. However, there are cases where you need some sort of direct access or should need to make some tailored IAM permission, where role the `User Access Administrator` comes in handy. The defaullt Azure AD roles include:

* **Readers**: Have read-only access for monitoring and oversight.
* **Developers**: Have contributor-level access to make changes within their environment.
* **Admins**: Have full contributor access, along with administrative privileges to manage IAM within their scope.

An additional layer of control is implemented through Altinn's use of AI-DEV and AI-PROD users when accessing Azure resources. The AI-DEV users are restricted to Dev and Test Subscriptions, while AI-PROD users are granted access to Staging and Prod Subscriptions. This separation is enforced as follows:

* **AI-DEV** users are assigned to AD Groups linked to Dev Management Groups, ensuring they only have access to Dev-related resources.
* **AI-PROD** users are assigned to AD Groups tied to Prod Management Groups, restricting their access to Staging and Prod resources.

**__NOTE:__** Should clarify distinction between AI-DEV and AI-PROD

**Azure AD group specification for Service Owners: TBD**

**Azure AD group specification for Altinn Capabilities**

| Group Name | Owners | Members | Management Group | Azure IAM role for Management Group |
| ---------- | ------ | ------- | ---------------- | -------------- |
| Altinn Capability {Name}: Readers Dev | Technical architect (AI-PROD) | AI-DEV users | Altinn-Capability-{Name}-Dev | `Reader` |
| Altinn Capability {Name}: Developers Dev | Technical architect (AI-PROD) | AI-DEV users | Altinn-Capability-{Name}-Dev | `Contributor` |
| Altinn Capability {Name}: Admins Dev  | Technical architect (AI-PROD) | AI-DEV users | Altinn-Capability-{Name}-Dev | `Contributor`, `User Access Administrator` |
| Altinn Capability {Name}: Readers Prod | Technical architect (AI-PROD) | AI-PROD users | Altinn-Capability-{Name}-Prod | `Reader` |
| Altinn Capability {Name}: Developers Prod | Technical architect (AI-PROD) | AI-PROD users | Altinn-Capability-{Name}-Prod | `Contributor` |
| Altinn Capability {Name}: Admins Prod | Technical architect (AI-PROD) | AI-PROD users | Altinn-Capability-{Name}-Prod | `Contributor`, `User Access Administrator` |

**Azure AD group specification for Altinn Products**

| Group Name | Owners | Members | Management Group | Azure IAM role for Management Group |
| ---------- | ------ | ------- | ---------------- | -------------- |
| Altinn Product {Name}: Readers Dev | Technical architect (AI-PROD) | AI-DEV users | Altinn-Product-{Name}-Dev | `Reader` |
| Altinn Product {Name}: Developers Dev | Technical architect (AI-PROD) | AI-DEV users | Altinn-Product-{Name}-Dev | `Contributor` |
| Altinn Product {Name}: Admins Dev  | Technical architect (AI-PROD) | AI-DEV users | Altinn-Product-{Name}-Dev | `Contributor`, `User Access Administrator` |
| Altinn Product {Name}: Readers Prod | Technical architect (AI-PROD) | AI-PROD users | Altinn-Product-{Name}-Prod | `Reader` |
| Altinn Product {Name}: Developers Prod | Technical architect (AI-PROD) | AI-PROD users | Altinn-Product-{Name}-Prod | `Contributor` |
| Altinn Product {Name}: Admins Prod | Technical architect (AI-PROD) | AI-PROD users | Altinn-Product-{Name}-Prod | `Contributor`, `User Access Administrator` |

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

# Drawbacks
[drawbacks]: #drawbacks

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

# Prior art
[prior-art]: #prior-art

# Unresolved questions
[unresolved-questions]: #unresolved-questions

# Future possibilities
[future-possibilities]: #future-possibilities