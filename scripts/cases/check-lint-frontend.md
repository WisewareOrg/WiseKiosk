# `check-lint-frontend`

The inputs this check has been run against, in both directions. What it *asserts* is
[`docs/CI.md`](../../docs/CI.md) § *Lint and type checks*'s; how to run a case is
[`../README.md`](../README.md)'s.

Each case is a `git archive` copy of the tracked tree at `28c5ca2`, with `frontend/node_modules`
symlinked in from the working tree and `just check-lint-frontend` (eslint, then svelte-check `--tsgo`,
both blocking) run inside the copy. eslint 10.10.0, typescript-eslint 8.69.0, eslint-plugin-svelte
3.23.0, svelte-check 4.7.6.

| Direction | Case | Input |
|---|---|---|
| Must fail | An unused variable | `regions.ts` gains `const __seedUnused = 1;`, never read — eslint's `@typescript-eslint/no-unused-vars` (recommended set) exits 1 naming the line, and `check-lint-frontend` exits 1 before svelte-check runs at all |
| Must pass | The same declaration, read | `__seedUnused` renamed `__seedUsed` and passed to `void __seedUsed;` — eslint exits 0 |
| Must fail | A prop-derived type mismatch in a `.svelte` file | `Clock.svelte` gains `const __seedMismatch: string = twentyFourHour;` (a `boolean`) beside a `void` read so eslint stays clean — `svelte-check --tsgo` exits 1 naming the line (plus a harmless `state_referenced_locally` warning from referencing a `$derived` value outside a closure, an artifact of the seed's own shape), and `check-lint-frontend` exits 1 too: svelte-check's line is unprefixed, so its exit code reaches the recipe's own. Plain `tsc` (`check-typecheck-frontend`) does not see this at all: a `.svelte` file's script block is not a source file bare `tsc` opens, which is why both gates exist rather than either alone |
| Must pass | The tree as it stands, a clean tree | `svelte-check --tsgo` reports zero findings against `ModuleEntry.component` and the whole tree, and `check-lint-frontend` exits 0 with both eslint and svelte-check blocking. [#275 resolve ModuleEntry.component prop-type variance so svelte-check blocks](https://github.com/WisewareOrg/WiseKiosk/issues/275) resolved the prop-type variance findings on `ModuleEntry.component` |
