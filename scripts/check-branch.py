#!/usr/bin/env python3
"""The branch name must follow type_number-snake_name (ADR 0006 rev 6): the shape is
defined once in scripts/branch-shape.regex, shared with the branch-shape pre-push hook;
the number resolves via the GitHub API to an open issue carrying a milestone
and exactly one type label, which is the branch type (ADR 0013 rev 4). main and
renovate/* are exempt. When an open PR exists, its Development field
(closingIssuesReferences) must link the branch's issue — any base; body
keywords only write the record against the default branch, so other bases
need the manual link. The PR's base and the issue's parent must agree: a
sub-issue means a shared merge target. Owner/repo come from the origin
remote, so the check works on any clone. GITHUB_TOKEN or GH_TOKEN is used as
a bearer token when set (rate limits, CI); the GraphQL phase requires it.

Dependencies: git. Standard library only (urllib, json).

What this has been run against, in both directions: cases/check-branch-py.md
"""

import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

SCRIPTS_DIR = Path(__file__).resolve().parent


def fail(message):
    print(f"check-branch: {message}", file=sys.stderr)
    sys.exit(1)


def read_patterns():
    """Each non-blank line of branch-shape.regex is a pattern; a name matches if any
    line does, which is what the one-answer guard below judges. A blank line is dropped
    rather than read: an empty pattern matches every name, and the guard cannot see one
    because a blank yields no type group."""
    text = (SCRIPTS_DIR / "branch-shape.regex").read_text()
    return [line for line in text.splitlines() if line.strip()]


def matches_shape(name, patterns):
    return any(re.search(p, name) for p in patterns)


def api_request(url, token, payload=None):
    """(status, parsed body) for a GitHub REST or GraphQL call; the body parses only
    on 200, which is the only status any caller reads past."""
    headers = {"Accept": "application/vnd.github+json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    request = urllib.request.Request(url, headers=headers, data=payload)
    try:
        with urllib.request.urlopen(request) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as e:
        return e.code, None
    except urllib.error.URLError as e:
        fail(f"cannot reach {url}: {e.reason}")


def is_number(value):
    return isinstance(value, (int, float)) and not isinstance(value, bool)


def main():
    if len(sys.argv) >= 2:
        branch = sys.argv[1]
    else:
        result = subprocess.run(
            ["git", "symbolic-ref", "--short", "-q", "HEAD"],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            fail("detached HEAD and no branch argument")
        branch = result.stdout.strip()

    if branch == "main":
        print("Branch 'main' is exempt: the mainline is not a work branch.")
        return
    if branch.startswith("renovate/"):
        print(f"Branch '{branch}' is exempt: Renovate names its own branches.")
        return

    patterns = read_patterns()
    if not matches_shape(branch, patterns):
        fail(
            f"branch '{branch}' does not match type_number-snake_name — type one of "
            "task|bug|design|module, number a GitHub issue number, name lowercase "
            "snake_case (e.g. task_27-process_gates; pattern: scripts/branch-shape.regex)"
        )
    branch_type = branch.partition("_")[0]
    number = int(branch.partition("_")[2].partition("-")[0])

    origin = subprocess.run(
        ["git", "remote", "get-url", "origin"], capture_output=True, text=True, check=True
    ).stdout.strip()
    slug_match = re.search(r"github\.com[:/]([^/]+/[^/]+)$", origin)
    slug = slug_match.group(1).removesuffix(".git") if slug_match else ""
    owner, _, repo = slug.partition("/")
    if not owner or not repo:
        fail(f"cannot derive owner/repo from origin remote '{origin}'")

    token = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN") or ""
    api_base = f"https://api.github.com/repos/{owner}/{repo}"

    status, issue = api_request(f"{api_base}/issues/{number}", token)
    if status == 404:
        fail(f"issue #{number} does not exist in {owner}/{repo}")
    if status != 200:
        fail(f"GitHub API returned {status} for {api_base}/issues/{number}")

    if "pull_request" in issue:
        fail(f"#{number} is a pull request, not an issue")
    state = issue.get("state")
    if state != "open":
        fail(f"issue #{number} is {state}, not open")
    label_names = [label["name"] for label in issue.get("labels", [])]
    if branch_type not in label_names:
        labels = ", ".join(label_names)
        fail(
            f"issue #{number} is not labeled '{branch_type}' (labels: {labels or 'none'}) "
            "— the branch type must match the ticket's template label"
        )

    type_groups = [
        m.group(1)
        for m in (re.fullmatch(r"\^\(([^)]+)\)_.*", line) for line in patterns)
        if m
    ]
    if len(type_groups) != 1:
        fail(
            f"scripts/branch-shape.regex yields {len(type_groups)} type group(s) — the "
            "set must have exactly one answer, and taking the first would let the count "
            "below read a group that does not govern this branch"
        )
    types = type_groups[0]
    if branch_type not in types.split("|"):
        fail(
            f"branch type '{branch_type}' is absent from the type set read out of "
            f"scripts/branch-shape.regex ('{types}') — the branch matched that pattern, "
            "so the extraction is wrong and the label count below would be meaningless"
        )
    type_labels = [name for name in label_names if name in types.split("|")]
    if len(type_labels) != 1:
        fail(
            f"issue #{number} carries {len(type_labels)} type labels "
            f"({', '.join(type_labels) or 'none'}) — exactly one of {types} names the "
            "template it was opened from, and a second makes the branch type ambiguous"
        )
    if issue.get("milestone") is None:
        fail(
            f"issue #{number} has no milestone — the milestone is this repo's phase "
            "axis, and a ticket outside it is absent from the definition of done it "
            "belongs to"
        )

    print(
        f"Branch '{branch}' links open issue #{number} "
        f"('{issue.get('title')}', labeled '{branch_type}')."
    )

    pr_env = os.environ.get("PR_NUMBER", "")
    if pr_env:
        status, pr = api_request(f"{api_base}/pulls/{pr_env}", token)
        if status != 200:
            fail(f"GitHub API returned {status} for {api_base}/pulls/{pr_env}")
    else:
        status, pulls = api_request(
            f"{api_base}/pulls?head={owner}:{branch}&state=open", token
        )
        if status != 200:
            fail(f"GitHub API returned {status} listing PRs for '{branch}'")
        pr = pulls[0] if pulls else None

    if pr is None:
        print(f"No open PR for '{branch}': the linkage gate binds once a PR is open.")
        return

    pr_number = pr["number"]
    base_ref = pr["base"]["ref"]
    default_branch = pr["base"]["repo"]["default_branch"]

    if not token:
        fail(
            f"PR #{pr_number} requires the recorded-linkage check, which needs GraphQL "
            "auth — export GH_TOKEN; a silently skipped gate is a false pass"
        )

    query = (
        "query($owner:String!,$repo:String!,$number:Int!,$issue:Int!)"
        "{repository(owner:$owner,name:$repo)"
        "{pullRequest(number:$number){closingIssuesReferences(first:20){nodes{number}}}"
        " issue(number:$issue){parent{number}}}}"
    )
    payload = json.dumps(
        {
            "query": query,
            "variables": {
                "owner": owner,
                "repo": repo,
                "number": pr_number,
                "issue": number,
            },
        }
    ).encode()
    status, reply = api_request("https://api.github.com/graphql", token, payload)
    if status != 200:
        fail(f"GitHub GraphQL returned {status}")
    if "errors" in reply:
        fail(
            "GitHub GraphQL errors: "
            + "; ".join(error["message"] for error in reply["errors"])
        )
    repository = reply.get("data", {}).get("repository") or {}
    pull_request = repository.get("pullRequest") or {}
    closing = (pull_request.get("closingIssuesReferences") or {}).get("nodes") or []
    if not any(node.get("number") == number for node in closing):
        hint = f"link it there (a 'Closes #{number}' body keyword writes the same record)"
        if base_ref != default_branch:
            hint = (
                "link it there manually — body keywords record nothing against "
                f"base '{base_ref}'"
            )
        fail(
            f"PR #{pr_number}'s Development field does not link issue #{number} — "
            f"{hint}; the gate reads GitHub's recorded state"
        )
    print(f"PR #{pr_number} records a closing reference to issue #{number}.")

    issue_node = repository.get("issue")
    parent_readable = (
        isinstance(issue_node, dict)
        and "parent" in issue_node
        and (
            issue_node["parent"] is None
            or is_number(issue_node["parent"].get("number"))
        )
    )
    if not parent_readable:
        fail(
            f"the GraphQL response carries no parent number for issue #{number} — the "
            "membership check read nothing, and a check that reads nothing must not "
            "report success. A present key with a null value is how 'no parent' "
            "arrives; anything else means the query stopped naming what is read below"
        )
    parent = "" if issue_node["parent"] is None else str(issue_node["parent"]["number"])
    if base_ref == default_branch:
        if parent:
            fail(
                f"issue #{number} is a sub-issue of #{parent}, but PR #{pr_number} "
                "targets the default branch — sub-issue membership means a shared merge "
                "target, not topical grouping; the milestone is what groups "
                "(ADR 0013 rev 4)"
            )
        print(
            f"Issue #{number} has no parent, and PR #{pr_number} targets the default "
            "branch."
        )
    else:
        if not matches_shape(base_ref, patterns):
            fail(
                f"PR #{pr_number}'s base '{base_ref}' is neither the default branch "
                "nor a conforming integration branch — an integration branch is a "
                "branch, so it links a ticket of its own (ADR 0006 rev 6)"
            )
        anchor = base_ref.partition("_")[2].partition("-")[0]
        if not parent:
            fail(
                f"PR #{pr_number} targets integration branch '{base_ref}' but issue "
                f"#{number} has no parent — a ticket whose PR targets an integration "
                f"branch is a sub-issue of that branch's anchor #{anchor} "
                "(ADR 0013 rev 4)"
            )
        if parent != anchor:
            fail(
                f"issue #{number} is a sub-issue of #{parent}, but PR #{pr_number} "
                f"targets '{base_ref}', which is anchored at #{anchor} — membership "
                "tracks the merge target"
            )
        print(
            f"Issue #{number} is a sub-issue of #{anchor}, which anchors base branch "
            f"'{base_ref}'."
        )


if __name__ == "__main__":
    main()
