# release-kit — full pre-publication code review

**Date:** 2026-07-13
**Scope:** the whole `github.com/burrowee-git/release-kit` library (7 packages), reviewed for public open-source release.
**Method:** three parallel review lenses — correctness+security, public-API+OSS-ergonomics, Go-idiom+portability+tests — each against `~/.claude/guidelines/CODE-REVIEW.md` + `lang/GO.md`. Findings consolidated + deduped here.
**Build evidence:** `go vet ./...` clean · `gofmt -s -l .` clean · `go test ./...` all 7 packages pass (12 tests) · `go.mod` has no `require` block, all imports stdlib except the module's own `sign` (zero-dep confirmed).

## Verdict

**0 Critical. Fundamentally sound and publishable after a hygiene pass.** The security-critical guarantees were verified, not assumed:

- **`vulncheck.Gate` is genuinely fail-closed** — every non-happy path (empty list, tool-not-found, mkdir fail, any non-zero govulncheck exit, treating vuln-found/error/bad-usage identically) returns non-nil; the report-write result is decoupled from the verdict. No path passes a vulnerable/errored scan.
- **`minisign.Verify` truly rejects tampered content** (round-trip test mutates post-sign and asserts failure), not just a missing signature.
- **No shell-injection surface** — all `exec.Command(name, args...)`; caller-supplied ldflags/package/identity are documented trust boundaries.
- **Portable** — every darwin/tool-dependent test `t.Skip`s correctly; `vulncheck` stubs `govulncheck` with a POSIX `sh` script (needs no real tool in CI). Would pass on a fresh Ubuntu runner.

The items below are latent footguns and OSS-polish, not active defects. **None blocks correctness; several are worth doing before the first `v0.x.y` tag** because they're breaking API changes that get expensive after publish.

---

## Important — recommend before publishing

Grouped: **(a) clear must-do hygiene**, **(b) breaking API calls best made pre-tag** (your decision).

### (a) Hygiene

**H1 · `build.GoWork` empty = workspace auto-discover, can diverge from what the gate scanned** — `build/build.go:68`
An unset `BinSpec.GoWork` is passed as `GOWORK=` (empty = go's *auto-discover*), so a build can silently enter workspace mode and resolve a **different module graph than `vulncheck.Gate` scanned** (the gate always forces `GOWORK=off`). For a CVE-gated release tool, "what was scanned ≠ what ships" is the integrity failure the whole library exists to prevent.
*Fix:* default an unset `GoWork` to `off` (mirror the gate); accept only `{"off",""→off}` or reject surprises. Document the default.

**H2 · `vulncheck` fail-closed rests on an unasserted govulncheck contract** — `vulncheck/vulncheck.go:47`
The gate trusts govulncheck's text-mode exit-3-on-findings. It breaks silently if (a) an ancient govulncheck (<v1) exits 0 with vulns, or (b) a future maintainer adds `-json`/`-format json` (JSON mode **always** exits 0 → every scan "passes").
*Fix:* assert a minimum govulncheck version before trusting exit codes, and add a guard comment at the `exec.Command(gv, "./...")` site: "exit-code-based; do NOT add -json/-format — it forces exit 0 and breaks fail-closed."

**H3 · `pack.Zip` drops the archive file's `Close()` error → silent truncated zip** — `pack/pack.go:31`
`zw.Close()` is checked, but the deferred `zf.Close()` (the syscall that actually persists bytes — ENOSPC/NFS) is discarded, so a corrupt/truncated release zip can report success.
*Fix:* close `zf` explicitly after `zw.Close()` and return its error (named-return capture idiom).

**H4 · Missing godoc on central exported types** — `build/build.go:16,29,38`, `version/version.go:16` (+ optional `sign` `Sign` methods)
`build.Target`, `build.Spec`, `build.Artifact`, `version.BumpKind` (+ its const block) have no doc comment → pkg.go.dev renders them bare, inconsistent with every other package header here.
*Fix:* add name-leading doc comments to each (and the const block).

**H5 · Security-relevant branches are untested** — `vulncheck.go:62-78`, `sign/sign.go:44`, `build/build.go:44-72`
The lowest-coverage code is the code that matters most: `vulncheck.resolveGovulncheck` (the LookPath→GOPATH→"not found" fallback that decides if the gate can even find its scanner) is never exercised; `AppleSigner.Sign`'s actual exec/error path is untested (only the pure `command()` builder is); and `build.Compile`'s documented "cross-compile darwin from Linux leaves it unsigned" guarantee (the thing that makes Linux-CI release builds safe) has no regression test.
*Fix:* add stub-driven tests (the repo's own `vulncheck_test.go` `writeStub` + `ToolPath`-stub patterns make all three trivial without real tools/credentials): resolution success + not-found error; `AppleSigner.Sign` via a stub `ToolPath` (success + error-wrap); and a foreign-`OS` Compile with a `Signer` stub that fails if invoked, proving the `host=="darwin"` guard gates the call.

### (b) Breaking API calls — best decided before the first tag

**B1 · `checksum.SumFile` is misnamed** — `checksum/checksum.go:17`
Reads as "sum one file"; actually hashes many files and writes a combined `SHA256SUMS`. Hard to change after a tag ships.
*Recommendation:* rename to `WriteSums` (or `SumsFile`) + update GUIDE §3. **Rec: do it.**

**B2 · `sign.AppleSigner.NotaryProfile` is a dead/leaky export** — `sign/sign.go:34`
Stored but never read; an adopter reasonably assumes setting it enables notarization, which the library explicitly does not do. (The earlier whole-branch review kept it as a "forward-looking field"; two independent public-lens reviewers now flag it as misleading on a public API.)
*Recommendation:* **drop it** (notarization is fully the product's job per the GUIDE) — closes the "does this notarize?" ambiguity. Reversing the earlier keep-decision is the right call for a public surface.

**B3 · No `context.Context` on exec-shelling functions** — `Compile`, `Gate`, `sign`/`minisign` `Sign`/`Verify`, `Stamp`
No caller can cancel or time-bound a hung `git`/`codesign`/`minisign`/`govulncheck`/`go build` (govulncheck can hang pulling the vuln DB; Compile loops an unbounded matrix). Adding `ctx` later is breaking.
*Recommendation:* add `ctx context.Context` as the first param now and use `exec.CommandContext`. **Worth doing pre-publication if you want it at all** — otherwise consciously accept "no cancellation" for v0.x.

**B4 · No runnable `example_test.go`** — repo root
The library's whole pitch is "primitives that compose"; there's no compilable worked example. An `Example` running the documented order (gate→stamp→build→checksum→minisign→pack) against a tiny temp module renders on pkg.go.dev **and** pins the GUIDE's ordering against drift.
*Recommendation:* add one. High-leverage adoption aid; not breaking, but belongs in the launch cut.

---

## Minor — polish (batch at leisure)

- **checksum/pack basename collisions** (`checksum.go:34`, `pack.go`): same-`<Name>` cross-arch artifacts silently produce duplicate lines/entries with different hashes. Detect duplicate basenames and error, or document "names must be unique." Note: `build.Compile` writes every target to `OutDir/<os>-<arch>/<Name>` — same basename per target — so this trap is one glue-mistake away.
- **`build.Paths([]Artifact) []string` helper** — every adopter hand-writes the `arts→[]string` glue for checksum/pack. One helper removes it.
- **`vulncheck` report-write error swallowed** (`vulncheck.go:51`) — the per-module `.txt` *is* the audit evidence; a silently-absent report undermines a "clean" cut's proof. Surface the write error.
- **`version.Bump` ignores `strconv.Atoi` overflow** (`version.go:40-42`) — regex guarantees digits, not magnitude; a 20-digit component parses to 0 and bumps wrong. Check the errors or bound the digit count.
- **`pack.Content.Name` unsanitized** (`pack.go:34`) — `../`/absolute names give a zip-slip-on-write archive. Trusted caller today; document the trust boundary or reject `..`/leading `/`.
- **unsigned-darwin-on-linux is silent** (`build.go:72`) — consider an `Artifact.Signed bool` so an orchestrator can refuse to ship an unsigned darwin binary on a public cut.
- **error hygiene:** inconsistent package prefixes; several bare `os` errors unwrapped in `checksum`/`pack`/`build` (stdlib `*PathError` carries the path, but `io.Copy`/`zip.FileInfoHeader` errors don't) — wrap with `%w` + context.
- **`vulncheck.trim`** (`vulncheck.go:80`) reinvents `strings.TrimSpace` (already used in `version.go`) — replace + delete the helper.
- **`checksum` `sort.Slice`** → `slices.SortFunc` (GO.md-preferred, `modernize`-flagged).
- **`.dp` brand jargon in the public GUIDE** (`GUIDE.md:19,40`) — genericize to "your encrypted/age-sealed secrets repo."
- **test-setup errors ignored** in `build_test.go`/`pack_test.go` (unlike checksum/version tests) — `t.Fatal` on setup for clear failures.
- **`Bump` default branch untested** — cheap regression if `BumpKind` is ever externally constructed.
- **No CI workflow / `.golangci.yml`** — out of scope to build here, but a public repo wants `.github/workflows` running `vet`/`test`/`gofmt -s -l` (+ `golangci-lint`: `errcheck`/`staticcheck`/`errorlint`/`errname` would auto-catch H3, M error-hygiene). Flag for the launch checklist.

---

## Confirmed good (explicit)

Fail-closed gate integrity · real tamper detection · no shell injection · zero third-party deps · gofmt/vet clean · correct cross-platform test skips (Linux-CI-ready) · MIT LICENSE present/correct · README shell-out list accurate · GUIDE function names/signatures + CVE-gate-first ordering accurate (only `NotaryProfile` muddies the "no notarization" scope).

## Suggested pre-publish sequence

1. Hygiene: **H1, H3, H4** (mechanical) → **H5** (tests) → **H2** (guard + version check).
2. API decisions (pre-tag): **B1 rename**, **B2 drop NotaryProfile**, **B4 example**; decide **B3 context** yes/no.
3. Launch checklist: add `.github/workflows/ci.yml` (vet/test/gofmt) + `.golangci.yml`.
4. Minors as a follow-up sweep.
