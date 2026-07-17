# release-kit — operating manual

Brand-agnostic, secret-free Go primitives for cutting **signed, checksummed, CVE-gated** release *artifacts*. Distribution and all secrets stay in each product's own `release` repo — this library only produces and verifies artifacts.

## Identity

- **Repo:** `burrowee-git/release-kit` (**PUBLIC**, published 2026-07-13) · module `github.com/burrowee-git/release-kit`
- **`gh.account`:** `burrowee-git`
- **Branch model:** trunk `main` (deploy/tag origin); code on `dev` worktree at `../.worktrees/dev`. Tags are the release surface — `v0.1.0` and `v0.1.1` shipped.
- **Stack:** Go 1.25, **no third-party deps**; shells out only to `go`, `git`, `codesign`, `minisign`, `govulncheck`.
- **Packages:** `version` · `build` · `sign` · `checksum` · `minisign` · `pack` · `vulncheck` (+ root `releasekit` doc package, `example_test.go`).
- **Consumers:** each product's `release` repo imports these and orchestrates in its own `cmd/release/main.go`. See [`GUIDE.md`](GUIDE.md) to stand up a new one; `Example_releaseFlow` in `example_test.go` shows the compose order.

## Design constraints (do not violate)

- **Secret-free & host-free.** No credential, hostname, bucket, or notarization submission belongs here — those are the consumer's job. Signing identities are injected (`sign.Signer`), never embedded.
- **Library, not framework.** Primitives the consumer composes; no orchestrator `main` lives here.
- **Fail-closed CVE gate.** `vulncheck.Gate` aborts before build on a reachable known CVE, no override.
- **Public API stability.** This is a published, imported library — treat exported signatures as contract; breaking changes need a version bump and a note in the release.

## Core principles

See [`DEVELOPMENT.md`](https://github.com/burrowee-git/resources/blob/main/docs/guidelines/DEVELOPMENT.md)
for the standard this code is written and reviewed against: think before coding,
simplicity first, surgical changes, verify before declaring done
(`GOWORK=off go build/vet/test ./...` must stay green — 30 tests / 8 packages).

## Task dispatch

Task-scoped work (coding, review, testing) runs as subagents; point the subagent
at the Guidelines table below.

## Guidelines

Canonical, shared across all Burrowee repos — read from `burrowee-git/resources`:

| Task | File |
|---|---|
| Contributing: branch → PR → review | [`docs/guidelines/WORKFLOW.md`](https://github.com/burrowee-git/resources/blob/main/docs/guidelines/WORKFLOW.md) |
| Principles · naming · architecture · errors · tests | [`docs/guidelines/DEVELOPMENT.md`](https://github.com/burrowee-git/resources/blob/main/docs/guidelines/DEVELOPMENT.md) |
| Code review compliance | [`docs/guidelines/CODE-REVIEW.md`](https://github.com/burrowee-git/resources/blob/main/docs/guidelines/CODE-REVIEW.md) |
| Traps that will bite you | [`docs/guidelines/TRAPS.md`](https://github.com/burrowee-git/resources/blob/main/docs/guidelines/TRAPS.md) |
| New here? | [`docs/onboarding/`](https://github.com/burrowee-git/resources/blob/main/docs/onboarding/README.md) |

Operator-only (machine-local, not required to contribute): release signing, deploy,
and the local repo registry live outside these repos and are not needed to write code
here.
