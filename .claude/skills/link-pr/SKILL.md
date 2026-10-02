---
name: link-pr
description: >-
  Record a WiseKiosk pull request's ticket in its Development field, from a session with no browser.
  Needed whenever the PR's base is an integration or epic branch rather than the default one, where a
  `Closes #N` body keyword writes nothing and the process gate fails on the missing record. Carries
  the GraphQL mutation `gh` has no subcommand for, the node-id lookups it takes, and the read-back
  that distinguishes a working call from a silent no-op. Invoke straight after `gh pr create` on a
  non-default base.
---

# Link a PR to its ticket

**`Closes #N` in a PR body writes the Development-field record only when the base is the default
branch.** A sub-issue's PR targets its integration branch
([`file-ticket`](../file-ticket/SKILL.md) § *Ordering, and epic membership* decides which tickets
those are), so on every such PR the keyword writes nothing while the body still reads as though it
did.

That is what makes the failure expensive rather than obvious: the body looks right, and
[`scripts/check-branch.py`](../../../scripts/check-branch.py) fails anyway with
`link it there manually — body keywords record nothing against base '<the base>'`. **The gate reads
GitHub's recorded state**, so no wording in the body will satisfy it. Requirement and rationale are
[`CONTRIBUTING.md`](../../../CONTRIBUTING.md)'s and
[ADR 0006 rev 6](../../../docs/decisions/0006-process-gates.md)'s; this skill is the mechanism.

**This skill is advisory.** Nothing forces its use. The half with teeth is the `process` job, which
refuses the merge later — so an unlinked PR is not lost, only expensive.

## Writing the record

There is a web UI for this — the **Development** panel on the pull request — and **no `gh`
subcommand**: nothing under `gh pr` writes it. `gh issue develop` is the near miss and is a different
thing, managing an issue's linked *branches* rather than a PR's closing reference, so it leaves the
gate exactly as red as it found it.

From a session without a browser the record is written with a GraphQL mutation, which takes **node
ids**, not numbers:

```sh
issue=$(gh api graphql -f query='{repository(owner:"WisewareOrg",name:"WiseKiosk"){issue(number:<n>){id}}}' \
  --jq '.data.repository.issue.id')
pr=$(gh api graphql -f query='{repository(owner:"WisewareOrg",name:"WiseKiosk"){pullRequest(number:<p>){id}}}' \
  --jq '.data.repository.pullRequest.id')
gh api graphql -f query="mutation{addCloseIssueReferences(input:{issueId:\"$issue\",pullRequestIds:[\"$pr\"]}){clientMutationId}}"
```

**Read it back rather than trusting the exit code** — the mutation returns a null
`clientMutationId` on success, so a call that worked and a call that did nothing print the same
thing:

```sh
gh api graphql -f query='{repository(owner:"WisewareOrg",name:"WiseKiosk"){pullRequest(number:<p>){closingIssuesReferences(first:5){nodes{number}}}}}' \
  --jq '.data.repository.pullRequest.closingIssuesReferences.nodes'
```

The ticket's number in that list is the whole of what the gate wants.

Undoing it is `removeCloseIssueReferences`, same input shape — the pair to reach for when a PR was
linked to the wrong ticket.

## When the mutation looks made up

It is not in `gh`'s help and both names are easy to doubt. They come from the live schema, and the
query that read them is how to check again if either is ever renamed:

```sh
gh api graphql -f query='{__type(name:"Mutation"){fields{name}}}' --jq '.data.__type.fields[].name' \
  | grep -i closeissue
```

## The rest of the PR's process obligations

Linking is one of several, and the others are not this skill's: the branch shape, the ticket's
labels, milestone and parent, and the Conventional-Commits PR title are
[`CONTRIBUTING.md`](../../../CONTRIBUTING.md) § *Tickets, branches, and titles*. Getting the ticket
itself into a state that can be branched on is [`file-ticket`](../file-ticket/SKILL.md)'s.
