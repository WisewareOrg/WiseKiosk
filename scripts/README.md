# What each check has been exercised against

The inputs each check has been run against, in both directions: the defect it must
catch, and the legal input spelled differently that it must not reject. **One file per check, under
[`cases/`](cases/)** — a reader touching one check reads that check's record and nothing else.

**What belongs in a case file, and what does not.** Four things — the cases in both directions, the
legal input the check rejects anyway, the gaps it is known to leave, and any provenance a case is
pinned to. A note beside a table says in **one line** what a case proves, where the row does not show
it. Anything longer is somewhere else: what a check asserts and why is [`docs/CI.md`](../docs/CI.md)'s,
and how a check works is its own header comment's.

**Why the record exists.** A check that reads nothing finds no violations and prints success, and a
check that rejects a legal spelling looks identical to one catching a real defect. Neither shows up in
a green run, so the only evidence a check works is the list of inputs somebody put through it.

**A check with no file here has no record** — not a claim that it is unverified, and not a claim that
it is verified. Nothing gates this: `just verify` grows and each new check arrives with no file and
nothing to say so. **#77 gate CI.md against the workflow does not close that** — it covers `CI.md`'s
sections against the workflow's jobs, not this record against the checks.

**A success line says how much was checked, not how much exists.** Only the first is evidence:
`17 tags over 5 elements and 6 relationships` reads the same whether every element carries a
requirement link or none does, where `18 tag applications on 7 of 11 elements and relationships` moves
when a link breaks. Nothing enforces this — a wrong success line fails no build.

| Check | Record |
|---|---|
| the workflow audit (`zizmor` + `actionlint`) | [cases](cases/workflow-audit.md) |
| `check-untracked.py` | [cases](cases/check-untracked-py.md) |
| `check-repo-silo.py` | [cases](cases/check-repo-silo-py.md) |
| `check-docs-index.py` | [cases](cases/check-docs-index-py.md) |
| `check-branch.py` | [cases](cases/check-branch-py.md) |
| `branch-shape.py` | [cases](cases/branch-shape-py.md) |
| `check-eol.py` | [cases](cases/check-eol-py.md) |
| `check-adr-index.py` | [cases](cases/check-adr-index-py.md) |
| `check-adr-revs.py` | [cases](cases/check-adr-revs-py.md) |
| `check-arch / splice-arch-diagrams.py` | [cases](cases/check-arch.md) |
| `check-arch-trace.py` | [cases](cases/check-arch-trace-py.md) |
| `check-go` | [cases](cases/check-go.md) |
| `check-fuzz` | [cases](cases/check-fuzz.md) |
| `check-lint-go` | [cases](cases/check-lint-go.md) |
| `check-vulns-go` (`scripts/vulns/check_vulns.py --scope go`) | [cases](cases/check-vulns-go.md) |
| `check-vulns-npm` (`scripts/vulns/check_vulns.py --scope npm`) | [cases](cases/check-vulns-npm.md) |
| `check-vulns-image` (`scripts/vulns/check_vulns.py --scope image`) | [cases](cases/check-vulns-image.md) |
| `check-lint-frontend` | [cases](cases/check-lint-frontend.md) |
| `check-typecheck-frontend` | [cases](cases/check-typecheck-frontend.md) |
| `check-secret-unwrap.py` | [cases](cases/check-secret-unwrap.md) |
| `check-dead-test.py` | [cases](cases/check-dead-test-py.md) |
| `check-render` (the framework's own specs under `frontend/tests/render/` and each module's own, beside its component) | [cases](cases/check-render.md) |
| `check-render-policy` (the render tier previewed under the served security response headers) | [cases](cases/check-render-policy.md) |
| `check-image` (the six harnesses it runs under `image/`) | [cases](cases/check-image.md) |
| `smoke-native` (the harness under `native/`) | [cases](cases/smoke-native.md) |
| `check-bringup` (`bringup/bring_up.py`) | [cases](cases/check-bringup.md) |
| `check-image-swap` (`bringup/image_swap.py`) | [cases](cases/check-image-swap.md) |
| the `draft` job's draft-release step (`publish/draft_release.py`) | [cases](cases/publish-draft.md) |
| the `verify` job's release checks (`publish/sbom_attest.py`, `publish/verify_metadata.py`, `publish/verify_release.py`) | [cases](cases/publish-verify.md) |
| `check-publish-permissions` (`publish/verify_permissions.py`) | [cases](cases/check-publish-permissions.md) |
| `check-restart-policy.py` | [cases](cases/check-restart-policy-py.md) |
| `check-site` | [cases](cases/check-site.md) |
| `the seven requirements-tree checks` | [cases](cases/the-requirements-tree-checks.md) |
| `report-proposed.py` | [cases](cases/report-proposed-py.md) |
| the Conventional-Commit gate (`commitlint`, both stages) | [cases](cases/commitlint.md) |
| `adr-rev-reach.py` | [cases](cases/adr-rev-reach-py.md) |
| CodeQL (first-party source scanning) | [cases](cases/codeql.md) |

## Running a case

A case is a throwaway repository, so it cannot pollute the tree under test and a seeded
credential-shaped string never reaches a remote. Run from the repository root, which is where the `cp`
reads from; each check's case writes that check's own input paths in place of the placeholders.

```sh
case_run () { # $1 label   $2 expect (pass|fail)   $3 file contents
  d=$(mktemp -d); mkdir -p "$d/<input dir>"
  printf '%s\n' "$3" > "$d/<input dir>/<input file>"
  cp scripts/<check> "$d/"       # before the add: several checks fail on an untracked file
  ( cd "$d" && git init -q . && git add -A )
  ( cd "$d" && <interpreter> <check> >/dev/null 2>&1 )
  [ $? -eq 0 ] && got=pass || got=fail
  [ "$got" = "$2" ] && echo "ok   $1" || echo "WRONG $1 (got $got)"
  rm -rf "$d"
}
```

Where a case is keyed on what the tree holds, extract a named commit rather than `HEAD`:

```sh
d=$(mktemp -d)
( cd "$REPO" && git archive <commit> | tar -x -C "$d" )
ln -s "$REPO/docs/requirements/.venv"        "$d/docs/requirements/.venv"
ln -s "$REPO/docs/architecture/node_modules" "$d/docs/architecture/node_modules"
cp "$REPO/scripts/<check>" "$d/scripts/"   # fresh: the archive carries that commit's copy
md5sum "$d/scripts/<check>"                # assert against the section's pin before reading a result
( cd "$d" && git init -q . && git add -A && git commit -qm fixture )   # citation scans read git ls-files
```

Eight standing traps, each hit at least once:

- **Confirm the seed applied before reading the result.** A seed that fails to land looks exactly like
  a working check. Seeding into a tree: `git diff --quiet <path>` first, and read a clean diff as the
  case failing.
- **Assert the property the case is about**, not that the text changed. Three seeds failed this way in
  one afternoon on `check-arch-trace.py` — one never wrote the file, one selected lines by indentation
  and moved 11 of 18, one used an input another case already satisfied. Each reported the verdict its
  row expected.
- **`git init` the scratch tree and run the script from inside it.** Several checks build their file
  list with `git ls-files`, and a root resolved with `git rev-parse --show-toplevel` climbs out of a
  bare directory or fails. Either way the check reads no files, or the wrong tree, and prints success
  — the fail-open this file exists to catch, wearing a passing case's costume.
- **Copy the script under test into the scratch tree**, not the other way round: the Python checks
  derive the tree they read from their own `__file__`.
- **Copy it in fresh each time and assert its md5.** A `git archive` fixture carries the scripts as
  they were at that commit, so a case run inside it exercises the *old* script and a fix appears to do
  nothing — three false "unfixed" results before this was noticed. The md5 assertion separates *the fix
  does not work* from *the fix was not in the tree you ran*. Pin the hash to a **commit that carries
  the script**, never a working tree: a hash paired with a commit whose tree holds a different script
  says nothing about which half is wrong.
- **Pin the tree as well as the script where a case counts anything.** A whole-tree seed counts what
  the directory or model holds, which the script's hash cannot see. Where a section states a baseline,
  reproduce it before trusting any figure below it.
- **Doorstop cases run against a copy of the tree, never the tree.** `doorstop --error-all` stamps a
  review fingerprint into any unstamped item, a mutation
  [`../CONTRIBUTING.md`](../CONTRIBUTING.md) says must never be cleared by re-running. All seven tree
  checks pass on one unseeded copy, which is what makes it a fixture rather than an approximation;
  `git status --short docs/requirements/` afterwards is the proof no case escaped.
- **A step added to a check must be re-run against every case the check already passed**, not only the
  case it was added for. Where the steps share mutable state a new one can undo what an existing one
  produces: on `check-arch` a `git add --intent-to-add` added to catch an uncommitted artifact
  re-opened the orphan case `HEAD` had closed, the two acting on one index in opposite directions
  ([cases/check-arch.md](cases/check-arch.md)). Delete each step and re-run every case to prove it
  necessary, recording the grid rather than a list of passing rows.

## Confirming a gate in CI rather than locally

Local runs prove the script; they do not prove the step is wired, reached, and able to fail the job.
For that, push a branch that adds its own name to `checks.yml`'s `push:` trigger — no pull request is
needed, and the `process` job skips itself on a push event — seed the defect, read the run, then push
the same branch with the seed removed and confirm it goes green.

Two things only a real run shows. A step that exits non-zero fails its job and the steps after it do
not run, so seeding two defects into one job shows the first gate to fail and hides the second — two
seeds want two pushes. And a seed placed in `pages.yml` is read by the checks without that workflow
ever executing, which is how a write grant can be tested without any run ever holding it.
