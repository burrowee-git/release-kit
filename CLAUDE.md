# release-kit — operating manual

Brand-agnostic, secret-free Go primitives for cutting **signed, checksummed, CVE-gated** release *artifacts*. Distribution and all secrets stay in each product's own `release` repo — this library only produces and verifies artifacts.

## Identity

- **Repo:** `burrowee-git/release-kit` (**PUBLIC**, published 2026-07-13) · module `github.com/burrowee-git/release-kit`
- **`gh.account`:** `burrowee-git`
- **Branch model:** trunk `main` (deploy/tag origin); code on `dev` worktree at `../.worktrees/dev`. Tags are the release surface — `v0.1.0` shipped.
- **Stack:** Go 1.25, **no third-party deps**; shells out only to `go`, `git`, `codesign`, `minisign`, `govulncheck`.
- **Packages:** `version` · `build` · `sign` · `checksum` · `minisign` · `pack` · `vulncheck` (+ root `releasekit` doc package, `example_test.go`).
- **Consumers:** each product's `release` repo imports these and orchestrates in its own `cmd/release/main.go`. See [`GUIDE.md`](GUIDE.md) to stand up a new one; `Example_releaseFlow` in `example_test.go` shows the compose order.

## Design constraints (do not violate)

- **Secret-free & host-free.** No credential, hostname, bucket, or notarization submission belongs here — those are the consumer's job. Signing identities are injected (`sign.Signer`), never embedded.
- **Library, not framework.** Primitives the consumer composes; no orchestrator `main` lives here.
- **Fail-closed CVE gate.** `vulncheck.Gate` aborts before build on a reachable known CVE, no override.
- **Public API stability.** This is a published, imported library — treat exported signatures as contract; breaking changes need a version bump and a note in the release.

## Core principles

Follow `~/.claude/CLAUDE.md` §Core principles verbatim: think before coding, simplicity first, surgical changes, goal-driven execution, verify before declaring done (`GOWORK=off go build/vet/test ./...` must stay green — 27 tests / 8 packages), document as you change.

## Task dispatch

Task-scoped work (coding, review, testing) runs as subagents; name the matching guideline file(s) below in the prompt so the subagent loads them first.

## Guidelines

| Task | File |
|---|---|
| Architecture / naming | `~/.claude/guidelines/ARCHITECTURE.md` |
| Testing (TDD) | `~/.claude/guidelines/TESTING.md` |
| Error handling | `~/.claude/guidelines/ERROR-HANDLING.md` |
| Build / deploy (release cuts) | `~/.claude/guidelines/BUILD-DEPLOY.md` |
| Apple signing | `~/.claude/guidelines/APPLE-SIGNING.md` |
| Code review | `~/.claude/guidelines/CODE-REVIEW.md` |
| GitHub CLI (`ghp`) | `~/.claude/guidelines/GITHUB.md` |
| Go overlay | `~/.claude/guidelines/lang/GO.md` |
