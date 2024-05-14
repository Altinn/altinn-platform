# Altinn Platform RFCs

[Platform RFCs]: #altinn-platform-rfcs

The "RFC" (request for comments) process is intended to provide a consistent
and controlled path for changes to the Altinn Platform (such as new features) so that all
stakeholders can be confident about the direction of the project.

Many changes, including bug fixes and documentation improvements can be
implemented and reviewed via the normal GitHub pull request workflow.

Note: this is plain a rip off of [Rust's RFC process](https://github.com/rust-lang/rfcs/)

## Table of Contents
[Table of Contents]: #table-of-contents

  - [Opening](#altinn-platform-rfcs)
  - [Table of Contents]
  - [When you need to follow this process]
  - [Before creating an RFC]
  - [What the process is]
  - [The RFC life-cycle]
  - [Reviewing RFCs]
  - [Implementing an RFC]
  - [RFC Postponement]
  - [Help this is all too informal!]
  - [Contributions]


## When you need to follow this process
[When you need to follow this process]: #when-you-need-to-follow-this-process

You need to follow this process if you intend to make "substantial" changes to
the Platform, or the RFC process itself. What constitutes a
"substantial" change is evolving based on community norms and varies depending
on what part of the ecosystem you are proposing to change, but may include the
following.

  - Any semantic or syntactic change to the platform that is not a bugfix.
  - Any task that is not upgrading/updating a component, ie. tasks that are maintenance.

Some changes do not require an RFC:

  - Rephrasing, reorganizing, refactoring, or otherwise "changing shape does
    not change meaning".
  - Additions that strictly improve objective, numerical quality criteria
    (warning removal, speedup, better platform coverage, more parallelism, trap
    more errors, etc.)

<!-- 
TODO: maybe for later?
If you submit a pull request to implement a new feature without going through
the RFC process, it may be closed with a polite request to submit an RFC first.
-->


<!-- 
TODO: not applicable ATM

### Sub-team specific guidelines
[Sub-team specific guidelines]: #sub-team-specific-guidelines

For more details on when an RFC is required for the following areas, please see
the sub-teams specific guidelines for:

  - [CI/CD changes]() -->

## Before creating an RFC
[Before creating an RFC]: #before-creating-an-rfc

<!-- A hastily-proposed RFC can hurt its chances of acceptance. Low quality
proposals, proposals for previously-rejected features, or those that don't fit
into the near-term roadmap, may be quickly rejected, which can be demotivating
for the unprepared contributor. Laying some groundwork ahead of the RFC can
make the process smoother. -->

Although there is no single way to prepare for submitting an RFC, it is
generally a good idea to pursue feedback from other project developers
beforehand, to ascertain that the RFC may be desirable; having a consistent
impact on the project requires concerted effort toward consensus-building.

The most common preparations for writing and submitting an RFC include talking
the idea over on our Github discussions, discussing the topic on
Slack, and occasionally coming from an existing Github issue.

As a rule of thumb, receiving encouraging feedback from long-standing project
developers, and particularly members of the relevant [sub-team] is a good
indication that the RFC is worth pursuing.


## What the process is
[What the process is]: #what-the-process-is

TBD

## The RFC life-cycle
[The RFC life-cycle]: #the-rfc-life-cycle

TBD


## Reviewing RFCs
[Reviewing RFCs]: #reviewing-rfcs

While the RFC pull request is up, the team may schedule meetings with the
author and/or relevant stakeholders to discuss the issues in greater detail,
and in some cases the topic may be discussed at a team meeting. In either
case a summary from the meeting will be posted back to the RFC pull request.

The team makes final decisions about RFCs after the benefits and drawbacks
are well understood. These decisions can be made at any time, but the sub-team
will regularly issue decisions. When a decision is made, the RFC pull request
will either be merged or closed. In either case, if the reasoning is not clear
from the discussion in thread, the sub-team will add a comment describing the
rationale for the decision.


## Implementing an RFC
[Implementing an RFC]: #implementing-an-rfc

Some accepted RFCs represent vital features that need to be implemented right
away. Other accepted RFCs can represent features that can wait until some
arbitrary developer feels like doing the work. Every accepted RFC has an
associated issue tracking its implementation in the correspondant repository; thus that
associated issue can be assigned a priority via the triage process that the
team uses for all issues in the repository.

The author of an RFC is not obligated to implement it. Of course, the RFC
author (like any other developer) is welcome to post an implementation for
review after the RFC has been accepted.

If you are interested in working on the implementation for an "active" RFC, but
cannot determine if someone else is already working on it, feel free to ask
(e.g. by leaving a comment on the associated issue).


## RFC Postponement
[RFC Postponement]: #rfc-postponement

TBD


### Help this is all too informal!
[Help this is all too informal!]: #help-this-is-all-too-informal

The process is intended to be as lightweight as reasonable for the present
circumstances. As usual, we are trying to let the process be driven by
consensus and community norms, not impose more structure than necessary.


### Contributions
[Contributions]: #contributions

TBD


[Slack channel]: 