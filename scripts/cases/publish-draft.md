# `publish-draft` (`scripts/publish/draft_release.py`)

What this script asserts, and why, is [`docs/CI.md`](../../docs/CI.md)'s § *Publishing and
provenance*; how to run a case is [`../README.md`](../README.md)'s.

Opened ahead of the script, under `#418 publish under immutable releases`'s test-first pairing:
every row below states the case, how it is seeded or run, and the expected outcome, with
**Evidence left empty**. A row's Evidence is filled once `scripts/publish/draft_release.py` exists
and that row has actually been run against it; nothing here is read as observed until then.

**Method.** A throwaway tag, `v0.0.99`, pushed to `WisewareOrg/WiseKiosk` — never published; its
draft release(s) and the tag itself are deleted once every row's Evidence is filled. Rows run in
the order listed, each row's precondition built on the state the row before it left, except the
published-release row, which runs read-only against the published `v0.2.0` rather than the
throwaway tag, and the `gh`-failure row, which is stateless and may run in any order. `TAG` and
`PRERELEASE` are read by the script from the environment, as the plan states; `GH_TOKEN`/`GH_REPO`
are read by `gh` itself, matching how `gh` is run by hand elsewhere in this tree.

| Case | Seed / run | Expected outcome | Evidence |
|---|---|---|---|
| No release carries the tag; `PRERELEASE=false` | tag `v0.0.99` pushed, no release exists on it — confirmed by `gh api --paginate repos/WisewareOrg/WiseKiosk/releases --jq '[.[] \| select(.tag_name == "v0.0.99")] \| length'` returning `0`; run `TAG=v0.0.99 PRERELEASE=false GH_REPO=WisewareOrg/WiseKiosk python3 scripts/publish/draft_release.py` | runs `gh release create v0.0.99 --draft --verify-tag --title v0.0.99 --generate-notes --prerelease=false`; exactly one release carries the tag — a draft, titled `v0.0.99`, non-empty generated notes, `isPrerelease` false | |
| Exactly one draft carries the tag, `isPrerelease=false`; re-run with `PRERELEASE=true` | the draft from the row above, unchanged; run `TAG=v0.0.99 PRERELEASE=true GH_REPO=WisewareOrg/WiseKiosk python3 scripts/publish/draft_release.py` | runs `gh release edit v0.0.99 --prerelease=true`; the same release id as the row above (no second release created), `isPrerelease` flips to true | |
| Exactly one draft carries the tag, `isPrerelease=true`; re-run with `PRERELEASE=false` | the draft from the row above, unchanged; run `TAG=v0.0.99 PRERELEASE=false GH_REPO=WisewareOrg/WiseKiosk python3 scripts/publish/draft_release.py` | runs `gh release edit v0.0.99 --prerelease=false`; the same release id as the two rows above, `isPrerelease` flips back to false — the other re-sync direction from the row above | |
| `--exclude-drafts` on image-swap's previous-release lookup | the draft from the row above carries the tag, `isDraft=true`, `isPrerelease=false` (prerelease held false so `--exclude-pre-releases` alone cannot also account for its absence); run `gh release list --repo WisewareOrg/WiseKiosk --exclude-pre-releases --exclude-drafts --json tagName --jq '[.[].tagName]'` and, separately, `gh release list --repo WisewareOrg/WiseKiosk --exclude-pre-releases --json tagName --jq '[.[].tagName]'` | the first list (`--exclude-drafts` present, per `publish.yml`'s image-swap `previous` step) does not contain `v0.0.99`; the second (the same query without it) does — the seeded defect `--exclude-drafts` fixes | |
| No release carries the tag; `PRERELEASE=true` | the draft from the rows above deleted (`gh release delete v0.0.99 --yes --repo WisewareOrg/WiseKiosk`, the tag ref left in place) so the tag is absent again; run `TAG=v0.0.99 PRERELEASE=true GH_REPO=WisewareOrg/WiseKiosk python3 scripts/publish/draft_release.py` | runs `gh release create v0.0.99 --draft --verify-tag --title v0.0.99 --generate-notes --prerelease=true`; one draft release carries the tag, titled `v0.0.99`, non-empty generated notes, `isPrerelease` true | |
| Exactly one release carries the tag, and it is published | no seed — run read-only against `v0.2.0`, an existing published (non-draft) release on `WisewareOrg/WiseKiosk`: `gh release view v0.2.0 --repo WisewareOrg/WiseKiosk --json body,assets,isDraft,isPrerelease` recorded first, then `TAG=v0.2.0 PRERELEASE=false GH_REPO=WisewareOrg/WiseKiosk python3 scripts/publish/draft_release.py` | exits 1; output carries `release v0.2.0 is already published and immutable; re-cut under a new version`; nothing written — the same `gh release view` read-back afterwards is byte-identical to the one taken before | |
| More than one release carries the tag | attempted via `gh api -X POST repos/WisewareOrg/WiseKiosk/releases -f tag_name=v0.0.99 -f draft=true -f name=v0.0.99-dup`, alongside the draft the row above left on `v0.0.99` — a draft release's `tag_name` names no git tag until it publishes, so a second draft naming the same tag is not expected to collide with the first; then run `TAG=v0.0.99 PRERELEASE=false GH_REPO=WisewareOrg/WiseKiosk python3 scripts/publish/draft_release.py`. If that `gh api` call is itself refused, the case is unseedable by this route — record the refusal here rather than the script's behavior, and this row stays without a script-side result | exits 1; output carries `more than one release carries tag v0.0.99; resolve by hand`; nothing written — the release count on the tag (two) and both releases' own fields are unchanged by the run | |
| A `gh` failure (bad token or repo) | run the script against a `gh` that cannot reach the release list at all — `GH_TOKEN` set to an invalid value, or `GH_REPO` pointed at a repository that does not exist, against `TAG=v0.0.99 PRERELEASE=false`: e.g. `TAG=v0.0.99 PRERELEASE=false GH_REPO=WisewareOrg/does-not-exist-00000 python3 scripts/publish/draft_release.py` | the release-lookup call (`gh api --paginate repos/{owner}/{repo}/releases`) fails before any branch is chosen; the script exits non-zero and its output carries `gh`'s own printed error text, unaltered — no case-specific text of this script's own authorship, since the plan ties this case to whatever `gh` itself prints rather than a fixed string; nothing written, there being no branch reached that would write | |

Cleanup, once every row above is run and its Evidence filled: the draft(s) left on `v0.0.99`
deleted (`gh release delete v0.0.99 --yes --repo WisewareOrg/WiseKiosk`, once per release object
the tag carries) and the tag itself deleted (`git push origin :refs/tags/v0.0.99`), confirmed by
`gh release list --repo WisewareOrg/WiseKiosk` and `git ls-remote --tags origin` each showing no
`v0.0.99`.
