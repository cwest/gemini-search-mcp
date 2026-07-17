# Spec: auto-propose the next SemVer tag after a feat/fix merges to main

## Problem

`release.yml` already runs GoReleaser on `push: tags: ['v*']` — the publish leg is
automated. But nothing cuts the tag after a feature merges, so a shipped
`feat:`/`fix:` sits unreleased until someone tags by hand. v0.2.0 shipped this way
(manually tagged after PR #6). The manual `git tag` is the entire gap.

## Approach

Add the **tag-proposal leg** in front of the existing publish leg using
[release-please](https://github.com/googleapis/release-please). On every push to
`main`, release-please ([`googleapis/release-please-action@v5`](https://github.com/googleapis/release-please-action))
computes the next SemVer bump from Conventional-Commit
history since the last release and opens/maintains a **release PR**. Merging that
PR cuts the `vX.Y.Z` tag, which fires the existing `release.yml` GoReleaser run.

This keeps Casey in the loop on the publish decision: the release PR is the gate —
review it, merge it, and the release happens. No manual `git tag` in the happy path.

### Why release-please over a bare post-merge tag-push workflow

The card offered two mechanisms. release-please is chosen because:

- It **maintains a human-reviewable release PR** (the tag decision stays a merge,
  matching Casey owning the publish gate) rather than auto-pushing a tag on every
  qualifying merge.
- It is the standard, well-maintained mechanism for Conventional-Commit → SemVer
  in a Go/GoReleaser repo, with `release-type: go` support and an accurate bump
  calculator (`feat:` → minor, `fix:` → patch, `feat!:`/`BREAKING CHANGE` → major).
- It maintains a `CHANGELOG.md`; GoReleaser's own `changelog: use: github` remains
  the source for the GitHub Release notes, so the two do not conflict.

## SemVer bump rules (release-please default)

| Commit type            | Bump   |
| ---------------------- | ------ |
| `fix:`                 | patch  |
| `feat:`                | minor  |
| `feat!:` / `fix!:`     | major  |
| `BREAKING CHANGE:` body| major  |
| `docs:`/`chore:`/etc.  | none   |

Pre-1.0 note: release-please's `bump-minor-pre-major` / `bump-patch-for-minor-pre-major`
knobs are left at defaults, so on `0.x` a breaking change bumps the minor and a
`feat:` bumps the patch — the conventional pre-1.0 behavior. Set `bump-minor-pre-major: true`
later if the repo wants `feat:` → minor while still on `0.x`.

## Files

1. `.github/workflows/release-please.yml` — runs release-please on `push: main`.
2. `release-please-config.json` — release-type `go`, root package, changelog on.
3. `.release-please-manifest.json` — seeds the current released version (`0.2.0`)
   so bumps are computed from that baseline forward, not from the repo's first commit.

## The token gotcha (load-bearing)

A tag pushed with the default `GITHUB_TOKEN` **does not trigger** other workflows —
GitHub deliberately suppresses events raised by `GITHUB_TOKEN` to prevent recursive
runs. If release-please used the default token, it would cut the `vX.Y.Z` tag but
`release.yml` would never fire, and the release would silently not publish.

Resolution: release-please authenticates with a repo-scoped **PAT** stored as the
`RELEASE_PLEASE_TOKEN` secret, falling back to `GITHUB_TOKEN` if the secret is
absent. When the PAT is present, the tag push it performs is attributed to a real
actor and correctly triggers `release.yml`.

- Required secret: `RELEASE_PLEASE_TOKEN` — a fine-grained PAT (or classic PAT with
  `repo`) that can push tags and open PRs on `cwest/gemini-search-mcp`.
- Fallback: if unset, the workflow still runs on `GITHUB_TOKEN`; the release PR is
  created and the tag is cut, but the operator must trigger `release.yml` once
  manually (or re-run it) for the very first release — documented in the PR.

## Verification

- `actionlint` passes on the new workflow.
- `release-please-config.json` and `.release-please-manifest.json` are valid JSON
  and validate against the release-please schemas.
- End-to-end dry-run documented in the PR body (the flow cannot be fully exercised
  until the workflow lands on `main` and a qualifying commit merges).

## Out of scope

- Changing `release.yml` / `.goreleaser.yaml` (the publish leg stays as-is).
- Provisioning the `RELEASE_PLEASE_TOKEN` secret (repo-admin action; documented).
