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

## Principles

See [`DEVELOPMENT.md`](https://github.com/burrowee-git/resources/blob/main/docs/guidelines/DEVELOPMENT.md)
for the standard this code is written and reviewed against: think before coding,
simplicity first, surgical changes, verify before declaring done
(`ci/run-tests.sh` must stay green — 59 tests + 1 example / 9 packages; the designed
skips are `TestRunTestsProductEqualsTheRegistryProduct`, which skips wherever `ci-lock` or
`CODING_ROOT` is absent (the dev box, burrowee-ci), and `TestSignVerifyRoundtrip`, which
skips where `minisign` is absent).

## Suite command and pre-land gate

`ci/run-tests.sh` is the suite command; `ci/run-tests.sh --help` is its reference.
It bundles the committed ref (default the current branch; uncommitted work is not
tested), clones it under `$HOME/ci-runs` on burrowee-ci, and runs `go test -count=1`
there with go1.26.6 from the module cache (`GOWORK=off`, `GOPROXY=off`) under
`ci-lock run burrowee-release-kit` — the product lock, taken exclusively, plus a shared
hold on the `burrowee` brand lock, so release-kit runs beside the other Burrowee products
(`product=` in the runner is a constant; `TestRunTestsProductEqualsTheRegistryProduct`
checks it against `ci-lock products --path` where `ci-lock` and `CODING_ROOT` are present,
and skips elsewhere). A lock not acquired exits 75; a lock not provisioned on the machine
(`ci-lock` exit 2: not provisioned, or usage) exits 3 naming `ci-lock install`
(operator). Each run's artifacts
land in a new `.ci-out/<short sha>.XXXXXX/` (gitignored; the path is printed), so
concurrent runs of one sha never share one. On the machine, `go test` runs in its own
process group, whose id is kept in the run's `pgid` file. An interrupt (Ctrl-C) first
kills the local ssh. It then sends that group TERM over one ssh and waits up to 10 s
for the group to exit. Only then does it remove the run's workdir, and `ci-lock`
releases the lock when its child ends. After `go test` returns, pass or fail, the runner
TERMs the recorded suite group and, if it has not exited within 10 s, KILLs it, so a
leaked child cannot hold the CI lock. An interrupted run's group gets TERM only.
`go test`'s scratch (`TMPDIR` and `GOTMPDIR`) is a fresh `mktemp -d /tmp/ci.XXXXXX` per
run, one scratch per run, shared by every stage, removed after every run, on an interrupt
once the group is stopped, and on a hangup (a dropped connection) by the runner itself
once the group is stopped. The template is repo-independent; the repo is named by the
run's workdir, whose `scratch` file records the path. Its headroom is computed, not
assumed: the longest `t.TempDir()` under it is 14 + 1 + 64 (name) + 10 (digits) + 5
(`/001/`) = 94 bytes, leaving 13 bytes for a unix socket name within Linux's 107 usable
`sun_path` bytes (108 with the NUL). `TestScratchLeavesSocketHeadroomForTheLongestTempDir`
asserts those 13 bytes and binds the 13-byte `headroom.sock` there, so any longer template
fails it. Never `go test` on a Darwin workstation.
`ci-test` compiles for linux here and then execs it.

Pre-land gate:

```sh
GOWORK=off GOOS=linux go vet ./...
GOWORK=off GOOS=linux go build ./...
ci/run-tests.sh --json ./...
~/.agents/scripts/comment-lint.sh --check .
test "$(ci-lock products --path .)" = "$(sed -n 's/^product=//p' ci/run-tests.sh)"   # workstation
```

The last line is the product check, a shell comparison on the workstation (no `go test`
runs there): the runner's `product=` constant must equal what `ci-lock products --path`
returns from the registry. `TestRunTestsProductEqualsTheRegistryProduct` makes the same
check in the suite and stays there, but it skips wherever `ci-lock` or `CODING_ROOT` is
absent, which is the dev box and burrowee-ci. The suite is `ci/run-tests.sh` on burrowee-ci.

`comment-lint --check` must exit 0: source carries no comments except the
directives Go tooling parses (hard rule 10).

The test-suite review runs:

```sh
ci/run-tests.sh --json --cover ./...        # counts with skips and failed packages (counts.txt), covered set (covered.txt)
ci/run-tests.sh --json ./... -- -shuffle=on # shuffled; seeds.txt has one `<package> <seed>` line per package
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

Review reports for this repo live in `burrowee-git/resources`, never in this repo: a project feature's review in its `docs/projects/<project>/features/<NN>-<slug>/review.md`, and a review in no project in `reviews/YYYY-MM-DD-<scope>-review.md` — for example the [2026-07-13 public review](https://github.com/burrowee-git/resources/blob/main/reviews/2026-07-13-release-kit-public-review.md).

Operator-only (machine-local, not required to contribute): release signing, deploy,
and the local repo registry live outside these repos and are not needed to write code
here.
