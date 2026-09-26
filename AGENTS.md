# release-kit — operating manual

Brand-agnostic, secret-free Go primitives for cutting **signed, checksummed, CVE-gated** release *artifacts*. Distribution and all secrets stay in each product's own `release` repo — this library only produces and verifies artifacts.

## Identity

- **Repo:** `burrowee-git/release-kit` (**PUBLIC**, published 2026-07-13) · module `github.com/burrowee-git/release-kit`
- **`gh.account`:** `burrowee-git`
- **Branch model:** trunk `main` (deploy/tag origin); code on `dev`, checked out beside `main` at `code/dev`; project worktrees under `code/.worktrees/<project-id>`. Tags are the release surface — `v0.1.0` and `v0.1.1` shipped.
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
(`ci/run-tests.sh` must stay green — 43 tests + 1 example / 9 packages).

## Suite command and pre-land gate

`ci/run-tests.sh` is the suite command; `ci/run-tests.sh --help` is its reference.
It bundles the committed ref (default the current branch; uncommitted work is not
tested), clones it under `$HOME/ci-runs` on burrowee-ci, and runs `go test -count=1`
there with go1.26.6 from the module cache (`GOWORK=off`, `GOPROXY=off`,
`TMPDIR=/tmp`, `GOTMPDIR` unset) under `ci-lock run burrowee`. Each run's artifacts
land in a new `.ci-out/<short sha>.XXXXXX/` (gitignored; the path is printed), so
concurrent runs of one sha never share one. On the machine, `go test` runs in its own
process group, whose id is kept in the run's `pgid` file. An interrupt (Ctrl-C) first
kills the local ssh. It then sends that group TERM over one ssh and waits up to 10 s
for the group to exit. Only then does it remove the run's workdir, and `ci-lock`
releases the lock when its child ends. Never `go test` on a Darwin workstation.
`ci-test` compiles for linux here and then execs it.

Pre-land gate:

```sh
GOWORK=off GOOS=linux go build ./... && GOWORK=off GOOS=linux go vet ./...
ci/run-tests.sh --json ./...
~/.agents/scripts/comment-lint.sh --check .
```

`comment-lint --check` must exit 0: source carries no comments except the
directives Go tooling parses (hard rule 10).

The test-suite review runs:

```sh
ci/run-tests.sh --json --cover ./...        # counts with skips and failed packages (counts.txt), covered set (covered.txt)
ci/run-tests.sh --json ./... -- -shuffle=on # shuffled; per-package seeds in seeds.txt
ci/run-tests.sh --json ./... -- -count=5    # repeated
```

The module cache on burrowee-ci is per account. A run as an account whose cache
lacks the go1.26.6 toolchain fails offline: seed it from the workstation, never by
putting a token on the machine.

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
