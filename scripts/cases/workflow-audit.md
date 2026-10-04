# The workflow audit: `zizmor` + `actionlint`

The inputs the adopted workflow audit has been run against, in both directions. The audit replaces
`check-workflow-hardening.mjs` (ADR 0016 rev 12; #105 adopt zizmor and actionlint), and the rows below
are that check's recorded cases re-run against the adopted pair — including the must-pass rows, since
an adopted tool may reject legal input the authored check accepted. What the gate asserts, and what it
deliberately lets through, is [`docs/CI.md`](../../docs/CI.md) § *Action pins and workflow privilege*'s.

**Tools, pinned as run:** `zizmor` 1.29.0, `--persona=pedantic`, offline (no token), input
`.github/workflows`; `actionlint` 1.7.12 with `shellcheck` 0.11.0 and `pyflakes` 3.4.0 on `PATH`
(the official image bundles both). CI runs the same versions from the digest-pinned images in
[`checks.yml`](../../.github/workflows/checks.yml). A case fails if either tool exits non-zero.
Locally the tools install without a container: `pip install zizmor==1.29.0`, and `actionlint` ships a
release binary.

**Run a case** with the [`../README.md`](../README.md) harness, both tools invoked in place of the
`node` line. The baseline fixture below is the defect-free case: it was proven to exit 0 in both
tools before any row was read, and each row seeds one change into it.

```yaml
name: w
on: push
permissions:
  contents: read
concurrency:
  group: w
jobs:
  j:
    name: j
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - run: echo hi
```

| Direction | Case | Seed | Caught by |
|---|---|---|---|
| Must fail | Unpinned action | `uses: actions/checkout@v7` | `unpinned-uses` (High) |
| Must fail | Expression as the reference | `uses: ${{ env.ACTION }}` | both: zizmor parse error; actionlint context error |
| Must fail | Container action on a tag | `uses: docker://alpine:3.19` | `unpinned-images` |
| Must fail | Reference with its value on the following line | `uses:` then the reference indented under it | `unpinned-uses` |
| Must fail | Flow-mapping step, unpinned | `- {uses: actions/checkout@v4, with: {x: 1}}` | `unpinned-uses` |
| Must fail | Second reference on a flow line | `steps: [{uses: …@<sha>}, {uses: …@v4}]` | `unpinned-uses` |
| Must fail | Quoted flow reference, unpinned | `- {uses: "actions/checkout@v4"}` | `unpinned-uses` |
| Must fail | Unpinned action after a block scalar ends | a `run:` block scalar, then `- uses: actions/checkout@v4` | `unpinned-uses` |
| Must fail | Unpinned action behind a dash-line scalar | `- if: >-` on the dash line, then `uses: …@v4` | `unpinned-uses` |
| Must fail | Write grant carrying a comment | `contents: write # seeded`, second grant below | `excessive-permissions` |
| Must fail | Quoted write level | `contents: 'write'` | `excessive-permissions` |
| Must fail | Blanket write | `permissions: write-all` | `excessive-permissions` |
| Must fail | Flow mapping granting write | `permissions: {contents: read, actions: write}` | `excessive-permissions` |
| Must fail | Multi-line flow mapping granting write | the same across three lines | `excessive-permissions` |
| Must fail | Unreadable line inside the block | a non-grant line under `permissions:` | both, as a YAML error |
| Must fail | No top-level block | the workflow declares none | `excessive-permissions` — the fixture answering ADR 0016 rev 12's question |
| Must fail | Block with nothing under it | `permissions:` then the next top-level key | both, as a schema error |
| Must fail | No workflow discovered | no file under `.github/workflows` | both: zizmor exits 3, `no inputs collected` |
| Must pass | SHA pin | `uses: actions/checkout@<sha> # v7.0.1` | exit 0 |
| Must pass | Quoted SHA pin | the same, reference in double quotes | exit 0 |
| Must pass | Digest-pinned container action | `uses: docker://alpine@sha256:<64 hex>` | exit 0 |
| Must pass | Repository-local action | `uses: ./.github/actions/local` | exit 0 |
| Must pass | Flow-mapping step, pinned | `- {uses: …@<sha>, with: {persist-credentials: false}} # v7.0.1` | exit 0 |
| Must pass | Apostrophe beside a pinned flow reference | `- {name: don't stop, uses: …@<sha>, with: {…}} # v7.0.1` | exit 0 |
| Must pass | No action at all | a job whose only step is `run:` | exit 0 |
| Must pass | Workflow fixture inside a heredoc | a `run:` block scalar containing `- uses: actions/checkout@v4` | exit 0 |
| Must pass | Comment on the block-scalar header | a comment after the block-scalar indicator, same body | exit 0 |
| Must pass | Pinned action behind a dash-line scalar | `- if: >-` with a real condition, then `uses: …@<sha> # vN` | exit 0 |
| Must pass | `uses:` named in an echoed string | `run: 'echo "pin it as - uses: owner/action@sha"'` | exit 0 |
| Must pass | `uses:` named in a trailing comment | `run: echo x # rewrites each - uses. line` | exit 0 |
| Must pass | `uses:` inside a `with:` value | `path: 'build - uses: cache'` under a pinned step | exit 0 |
| Must pass | `uses:` inside an `env:` value | `NOTE: "a, uses: b"` | exit 0 |
| Must pass | Empty flow mapping | `permissions: {}` | exit 0 |
| Must pass | Quoted scope key | `"contents": read` | exit 0 |
| Must pass | Comment inside the block | a comment line between `permissions:` and its grant | exit 0 |
| Must pass | Block placed after `jobs:` | the top-level block declared below the jobs | exit 0 |

Three notes the rows cannot carry alone:

- **Retired, deliberately: the version-comment row.** The recorded must-fail *pinned, no version
  comment* passes the adopted pair — the `# vN` obligation is retired outright by ADR 0016 rev 12.
  zizmor's `ref-version-mismatch` audit covers the comment truthfully, but only online; the gate runs
  the offline set.
- **Legal input `pedantic` rejects**, the adopted counterpart of the retired check's known-rejections
  table. `permissions: read-all` fails `excessive-permissions` (the disagreement ADR 0016 rev 12
  records; no suppression is written because no workflow uses it), and any grant other than a bare
  `contents: read` — read grants and job-level elevations included — fails `undocumented-permissions`
  until it carries an adjacent explanatory comment. The live workflows carry those comments.
- **Two recorded must-pass spellings were never parseable YAML**, and one was not a valid step. The
  recorded *echoed string* and *`with:` value* inputs put `- uses: x` after a colon inside a plain
  scalar, which YAML refuses (`mapping values are not allowed in this context` — actionlint's own
  parse error); the recorded apostrophe-flow row carried `msg:` and `other:` keys no step schema
  accepts. The retired text scanner passed all three as lines; the adopted parsers reject them, and
  the legal respellings of the same intent in the rows above exit 0. The retired check's three
  known-rejections rows (`env:` flow maps and a flow step whose `name:` contains `uses:`) all pass
  the adopted pair.

**Fail-direction in CI**, per [`../README.md`](../README.md) § *Confirming a gate in CI*, on
since-dropped push-trigger commits: a seeded unpinned action failed the zizmor step (run
31927598981); a seeded undefined `needs:` failed the actionlint step with the zizmor step green (run
31927645635) — GitHub separately refused that seeded `pages.yml` in a workflow-file run of its own.
