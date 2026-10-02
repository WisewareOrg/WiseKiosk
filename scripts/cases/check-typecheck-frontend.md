# `check-typecheck-frontend`

The inputs this check has been run against, in both directions. What it *asserts* is
[`docs/CI.md`](../../docs/CI.md) § *Lint and type checks*'s; how to run a case is
[`../README.md`](../README.md)'s.

Each case is a `git archive` copy of the tracked tree at `0a34e62`, the commit carrying
`check-typecheck-frontend`, with `frontend/node_modules` symlinked in from the working tree and
`node_modules/@typescript/native/bin/tsc --noEmit -p tsconfig.json` run inside `frontend/`. TypeScript
7.0.2 (the `@typescript/native` alias), against what the recipe invokes by explicit path.

| Direction | Case | Input |
|---|---|---|
| Must fail | An undefined identifier | `regions.ts` gains `export const __seedUndefined = undefinedIdentifier;` — `tsc` exits 1 with `TS2304: Cannot find name 'undefinedIdentifier'` |
| Must pass | The same declaration, bound to a real value | `undefinedIdentifier` replaced by `1` — `tsc` exits 0 |
| Must pass | The tree as it stands | — |

**What this does not cover.** Component-prop variance on `.svelte` files — the kind
`check-lint-frontend`'s svelte-check half catches — does not reproduce here: plain `tsc` type-checks a
`.svelte` import through Svelte's own ambient module declaration, which is looser than svelte-check's
own preprocessing, so this gate's population and `check-lint-frontend`'s overlap on `.ts` files but
diverge on `.svelte` ones. [#275 resolve ModuleEntry.component prop-type variance so svelte-check
blocks](https://github.com/WisewareOrg/WiseKiosk/issues/275) resolved three such findings on
`ModuleEntry.component`, which this gate would not have caught either way, for the reason above.
