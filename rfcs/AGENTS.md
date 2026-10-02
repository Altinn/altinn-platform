# AGENTS.md

This file applies to all files in `rfcs/`.

## Write Simplified Technical English

All RFC text must follow ASD-STE100 (Simplified Technical English).

- Write short sentences. Use a maximum of 20 words in an instruction and 25 words in a description.
- Put one topic in each sentence. Put one instruction in each sentence.
- Use the active voice and the present tense.
- Use one word for one thing. Do not use synonyms for the same thing.
- Use common words. Write an acronym in full the first time you use it.
- Write "must" for a requirement and "do not" for a prohibition. Do not write "should".
- Keep the articles "a" and "the". Do not remove them to make a sentence shorter.
- Do not use idioms, metaphors or marketing words.
- Do not describe a design as "<product>-style". Name the mechanism instead.
- Use a list for steps and for parallel items. Use a table for data.
- Apply these rules to new text. Do not rewrite a merged RFC only to apply them.

## When an RFC is necessary

- Write an RFC for a substantial change to the platform or to the RFC process. A substantial change adds a feature or changes behaviour, and is not a bugfix.
- Do not write an RFC for a bugfix, an upgrade, maintenance, a refactor, or a change that only improves a measurable value.
- Get feedback before you write. Use GitHub Discussions, Slack or an existing issue.

## How the process works

1. Copy `0000-template.md` to `NNNN-<feature_name>.md`. `NNNN` is the next free number. Check the merged files and the open RFC pull requests, because an open pull request also claims a number.
2. Fill in the header: `Feature Name` (snake_case), `Start Date`, `RFC PR`, `Github Issue`, `Product/Category` and `State`. `Title` and `Github Discussion` are optional.
3. Set `State` to **REVIEW**. The team changes it to **ACCEPTED** or **REJECTED** when it decides.
4. Open a pull request with the title `docs(rfcs): RFC NNNN <title>`. Put the pull request link in the header when you know it.
5. The team reviews in the pull request. Notes from meetings go into the pull request as comments.
6. An accepted RFC gets a tracking issue for the implementation. The author does not have to implement it.

## Template sections

Keep all sections of the template, in this order, with their anchor lines (for example `[summary]: #summary`).

| Section | Content |
|---|---|
| Summary | One paragraph. |
| Motivation | Why we do this, which use cases it supports, and the expected outcome. |
| Guide-level explanation | Teach the feature as if it already exists. Use examples. Name new concepts. |
| Reference-level explanation | The technical design: interactions, implementation and corner cases. |
| Drawbacks | Why we must not do this. |
| Rationale and alternatives | Why this design, which alternatives, and the cost of not doing it. |
| Prior art | What other projects did, good and bad. |
| Unresolved questions | What the RFC, the implementation and later work must resolve. |
| Future possibilities | Natural extensions and out-of-scope ideas. Not a reason to accept the RFC. |

Write "None." in a section if you have nothing to say. Do not delete the section.

## Checks

There is no CI for `rfcs/`. Before you finish, read the text once more against the rules above and check that all links in the header resolve.
