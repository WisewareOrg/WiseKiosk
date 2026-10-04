# `check-lint-go`

The inputs this check has been run against, in both directions. What it *asserts* is
[`docs/CI.md`](../../docs/CI.md) § *Lint and type checks*'s; how to run a case is
[`../README.md`](../README.md)'s.

Each case is a `git archive 47a1742` copy of the tracked tree — the commit this change forked from
— with this change's `backend/.golangci.yml`, `backend/internal/boundary/doc.go`,
`backend/oapi-codegen.yaml` and `justfile` copied in fresh over it, seeded in place, and `just
check-lint-go` run inside the copy. golangci-lint 2.13.2, built with Go 1.26.5, against the default
linter set (errcheck, govet, ineffassign, staticcheck, unused) plus `depguard`. **The three rows
below naming `github.com/google/uuid`** additionally run `go get github.com/google/uuid@v1.6.0` in
the copy before the import is seeded, so `go.sum` carries the entry the seed needs to compile —
without it the seed fails at `typecheck` on a missing `go.sum` entry rather than being reported by
`depguard`.

| Direction | Case | Input |
|---|---|---|
| Must fail | An unchecked error return | `health.go`'s `GetHealthz` writes a response body with `w.Write([]byte("ok"))`, its `(int, error)` return discarded — `errcheck` exits 1 naming the line |
| Must pass | The same write, error explicitly discarded | `w.Write(...)` rewritten `_, _ = w.Write(...)` — the legal spelling every fix in this PR uses, `errcheck` reports nothing |
| Must pass | The tree as it stands | — |
| Must fail | A non-standard-library import in a hand-written file | `github.com/google/uuid` imported and used in `health.go` — `depguard` exits 1 naming the line, confining the `main` rule's allow-list to `$gostd` and this module's own packages |
| Must fail | The same import added by hand to the generated file | `github.com/google/uuid` imported and used in `boundary.gen.go` after generation — `depguard` exits 1 naming the line; the five linters excluded from `*.gen.go` by path (errcheck, govet, ineffassign, staticcheck, unused) report nothing on the same file, so depguard is the only linter that still inspects it |
| Must pass | The same import in a test file | `github.com/google/uuid` imported and used in a new `_test.go` file — the `main` rule excludes `$test`, `depguard` reports nothing |
| Must pass | An unchecked error return in the generated file | `func seedUnchecked() { os.Remove("") }` added to `boundary.gen.go` — `errcheck` is excluded from `*.gen.go` by path, reports nothing |
| Must fail | The same unchecked error return, hand-written | the same `seedUnchecked` function added to `health.go` — `errcheck` exits 1 naming the line, the pairing that shows the generated-file exclusion for the five linters is a path-scoped rule rather than an engine-wide skip |

**Known gaps.** `golangci-lint`'s own default `max-same-issues: 3` caps identical-message findings at
three per run: a fourth `file.Close` errcheck finding in `staticserve.go` was hidden behind three
already-reported occurrences on the first real run against this tree — **13 findings total, not the
12 first measured** (11 errcheck, 2 staticcheck) — and only surfaced once those three were fixed.
Nothing here raises the cap or asserts against it — a config widening it is a rule change beyond the
default set this decision adopted, and the finding it would have caught either shows up once the
issues ahead of it clear, as this one did, or is caught at review.

The five linters re-excluded from `*.gen.go` (errcheck, govet, ineffassign, staticcheck, unused) are
named individually in `backend/.golangci.yml` and matched against generated files by the `.gen.go`
suffix, not derived from the default linter set at run time. A future `golangci-lint` bump that adds
a linter to `standard` would report on generated code rather than being silently excluded alongside
the five — a loud finding on the next bump, not a silent pass, but a gap this config does not close
on its own.

Of the five, only `errcheck`'s exclusion is seeded against the generated file (the unchecked error
return seeded in the generated file, then hand-written, above). `govet`, `ineffassign`,
`staticcheck` and `unused` are not seeded against the generated file for this case; their
exclusion's effect there is not demonstrated by a seed.
