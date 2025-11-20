- Feature Name: release_please
- Start Date: 2025-11-19
- RFC PR: [altinn/altinn-platform#0000](https://github.com/altinn/altinn-platform/pull/2549)
- Github Issue: [altinn/altinn-platform#0000](https://github.com/altinn/altinn-platform/issues/2549)
- Product/Category: Release management
- State: **ACCEPTED** (possible states are: **REVIEW**, **ACCEPTED** and **REJECTED**)

# Summary
[summary]: #summary
[Release please](https://github.com/googleapis/release-please) automates CHANGELOG generation and Github Releases creation.
We already have agreed upon using [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/#summary) adding release please adds more value to that paradigm

# Motivation
[motivation]: #motivation

The repo is growing with a lot of small software pieces that we update and release independently. Keeping track of all changes and what pieces have changed since their last release is getting harder. Release Please will make it simpler by creating PR(s) that can be merged once we are ready to release.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation

Release Please will automate the release process for you. When you merge changes to `main` using [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/#summary), Release Please will automatically create a pull request with updated version numbers, a changelog, and a GitHub Release.

To trigger a new release, simply merge the Release Please pull request. This means you no longer need to manually update version numbers or write changelogs. Just focus on your code, use conventional commits, and Release Please handles the rest.

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

This section details the technical implementation of Release Please within our GitHub repositories.

Release Please operates by scanning commit history for [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/#summary). When a commit indicates a new version (e.g., `feat:` for a minor version, `fix:` for a patch version, `BREAKING CHANGE:` for a major version), Release Please will:

1.  **Create a Release Pull Request:** A new pull request will be opened against the `main` branch (or the configured release branch). This PR will contain:
    *   **Updated Version Number:** The `package.json`, `pom.xml`, or other configured version files will be updated to the new semantic version.
    *   **CHANGELOG.md:** A `CHANGELOG.md` file will be generated or updated, containing a summary of all changes since the last release, categorized by commit type (features, bug fixes, etc.).
    *   **GitHub Release Draft:** A draft GitHub Release will be created, pre-populated with the changelog entries.

2.  **Triggering a Release:** Merging the Release Pull Request will:
    *   **Publish GitHub Release:** The draft GitHub Release will be published, making the new version and its changelog publicly available.
    *   **Tag Creation:** A corresponding Git tag (e.g., `v1.2.3`) will be created on the merged commit.

# Drawbacks
[drawbacks]: #drawbacks

- It introduces another automated process that needs to be monitored and maintained.
- It requires strict adherence to conventional commit messages, which might be a learning curve for some contributors.
- The automated nature might reduce human oversight on release notes, potentially leading to less curated or less user-friendly release descriptions if not managed carefully.


# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

Release please is a well-established tool in the open-source community, and it integrates seamlessly with conventional commits, which we already use. This makes it a natural fit for our existing workflows.

Alternative approaches would involve custom scripting for changelog generation and release creation, which would require significant development and maintenance effort. Given the maturity and widespread adoption of Release Please, building a custom solution would be reinventing the wheel and would likely be less robust and feature-rich.

Not implementing Release Please would mean continuing with manual version bumping and changelog generation, which is prone to human error, inconsistent, and time-consuming. It would also hinder our ability to easily track and communicate changes across our growing number of independent software pieces.

# Prior Art
[prior-art]: #prior-art

Release Please is widely used across many open-source projects, particularly those under the Google umbrella (e.g., Angular, Kubernetes, many Google Cloud SDKs). Its adoption by such large and active communities demonstrates its effectiveness and reliability in managing releases for complex projects with numerous contributors. The positive experience of these communities, especially in maintaining consistent versioning and detailed changelogs, serves as strong prior art for its utility in our context.

# Unresolved questions
[unresolved-questions]: #unresolved-questions

# Future possibilities
[future-possibilities]: #future-possibilities
