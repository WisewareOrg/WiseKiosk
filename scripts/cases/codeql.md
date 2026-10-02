# CodeQL

The inputs the first-party source scan has been run against, in both directions. What it *asserts*,
and why, is [`docs/CI.md`](../../docs/CI.md) § *First-party source scanning*'s. CI-only: no local
recipe reproduces it, per § *Gate wiring*'s no-local-form list.

**Pins, as run:** `github/codeql-action` `v4.37.9` (`init`, `autobuild`, `analyze`), pinned to its
release commit in [`../../.github/workflows/codeql.yml`](../../.github/workflows/codeql.yml);
`actions/checkout` `v7.0.1`, the pin every workflow in this repository shares.

**Must pass:** this gate's own `codeql` job, green over all four languages, on the PR this file was
added in.

**Must fail (fail-direction in CI)**, per [`../README.md`](../README.md) § *Confirming a gate in CI
rather than locally*: a throwaway branch, `seed/codeql-2026-09-05`, cut from this PR's branch so
`codeql.yml` was present, carrying one canonical vulnerable pattern per language in ordinary source
files (not `scripts/gate-fixtures/` — [ADR 0010 rev 3](../../docs/decisions/0010-runtime-materialised-gate-fixtures.md)'s
runtime-materialised-fixture convention is for a *standing* meta-gate, which this repository does not
build for CodeQL; a throwaway branch, opened as a draft pull request and closed unmerged, is how
every other gate's fallibility is proven here — [`docs/CI.md`](../../docs/CI.md) § *Generated
boundary contract*). Draft PR #282 against `main`, closed unmerged, branch deleted after recording:

| Language | Rule id | Pattern | Alert title | Run |
|---|---|---|---|---|
| Actions | `actions/code-injection/critical` | untrusted `github.event.issue.title` interpolated into a `run:` step | "Code injection" (critical) | [33968978796](https://github.com/WisewareOrg/WiseKiosk/actions/runs/33968978796) |
| Go | `go/path-injection` | an `net/http` request query parameter passed to `os.ReadFile` | "Uncontrolled data used in path expression" (high) | [33968978796](https://github.com/WisewareOrg/WiseKiosk/actions/runs/33968978796) |
| Svelte/TypeScript | `js/xss` | `location.hash` assigned to `Element.innerHTML` | "Client-side cross-site scripting" (high) | [33968978796](https://github.com/WisewareOrg/WiseKiosk/actions/runs/33968978796) |
| Python | `py/command-line-injection` | an `http.server` request path passed to `subprocess.run(..., shell=True)` | "Uncontrolled command line" | [33969123713](https://github.com/WisewareOrg/WiseKiosk/actions/runs/33969123713) |

**Two runs, not one, because the first Python pattern proved nothing.** The branch's first push used
`sys.argv` as the tainted value; CodeQL's default Python threat model does not treat a command-line
argument as untrusted (only a genuinely remote source is, absent an extended threat-model
configuration), so run [33968978796](https://github.com/WisewareOrg/WiseKiosk/actions/runs/33968978796)
raised the Actions, Go and Svelte/TypeScript alerts above plus one unintended medium finding
(`seed-vuln.yml` had declared no `permissions:` block) and nothing for Python — a case that would
have read as a passing seed had the row not been checked against the alert list rather than against
the job's exit status alone. The fixup push rewrote the Python fixture around an `http.server`
request handler (a recognised remote source) and added the missing `permissions:` block; run
[33969123713](https://github.com/WisewareOrg/WiseKiosk/actions/runs/33969123713) raised the Go,
Svelte/TypeScript and Python alerts above, and no unintended finding. The Actions alert did not
reappear in this run's own "new alerts" list — see *Known gaps* below for why that is read as the
diff view's own behaviour rather than the pattern no longer firing.

**Known gaps.**

- **No case proves a seed *stops* raising its alert once fixed.** Every other recorded gate in this
  repository pairs its must-fail row with a must-pass row over the same seed, bumped or corrected
  ([`check-arch.md`](check-arch.md)'s pattern). The throwaway branch was deleted after one
  observation in each direction rather than pushed a third time with the vulnerable lines removed,
  because the four rows above already demonstrate the analysis reaches and reports each pattern —
  what is unverified is only that removing the pattern silences the specific alert rather than some
  unrelated one.
- **The severity-threshold setting is not exercised here.** Whether `GitHub Advanced Security`'s
  `CodeQL` check actually fails the pull request at every severity present above (critical and high;
  no medium or low pattern was seeded) is the ruleset setting [`docs/CI.md`](../../docs/CI.md)
  § *Gate wiring* records, read back on the ticket rather than reproduced by a seed.
- **Two runs on one branch, not independent seeds.** The Go and Svelte/TypeScript patterns were
  unchanged between the two pushes, and their alerts appeared in both runs (33968978796 and
  33969123713 alike) — reproduction of the same seed, not two independent confirmations. A false
  positive from an unrelated cause would have reproduced identically both times too. The Actions
  alert appeared only in the first run and not the second, most likely because GitHub's per-PR
  "new alerts" view does not re-list a finding already reported for unchanged content on an earlier
  run of the same pull request, rather than the pattern having stopped firing.
