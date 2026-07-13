# Stand up a release-kit for a new product

`release-kit` gives you signed, checksummed, CVE-gated release *artifacts*. Your
product's release repo owns *distribution* (where they go) and all secrets.

## 1. Import

In your product's `release` repo:

```
go get github.com/burrowee-git/release-kit@latest
```

## 2. Config (checked-in, non-secret)

Describe your product in a Go struct or a `release.toml`: brand, per-component
source dirs + packages + ldflags, version scheme, signer selection, target
os/arch matrix, and the **path** to your minisign key (the key itself stays
age-sealed in your encrypted secrets repo — never commit it).

## 3. `cmd/release/main.go` — orchestrate

Call the primitives in order, threading one `context.Context` through the
whole run so a hung subprocess (`git`/`codesign`/`minisign`/`govulncheck`/`go
build`) can be canceled or time-bounded. The CVE gate is MANDATORY on public
cuts:

1. parse flags / prompt (`--public-release` = sign + gate; `--vulncheck`; `--apple`);
2. `vulncheck.Gate(ctx, modules, opts)` — **first, on every public cut** (fail-closed);
3. `version.Stamp(ctx, ...)` → `build.Compile(ctx, spec)` → `checksum.WriteSums(build.Paths(arts), out)` → `minisign.Sign(ctx, ...)` → `pack.Zip(spec)`
   (`build.Paths` turns the `[]build.Artifact` from `Compile` into the `[]string`
   `checksum.WriteSums` wants; `pack.Zip` takes its own `[]pack.Content` instead);
4. your distribution (GitHub Release, R2, host upload, catalogs, bootstraps).

## 4. Signing identity is yours

Construct a `sign.Signer` from your config — `sign.AdHocSigner{}` for dev/CI, or
`sign.AppleSigner{Identity: "...", ToolPath: "..."}` for a real Developer ID
release. Nothing about the identity lives in release-kit.

## 5. Minisign key

release-kit expects a **password-less** minisign secret key (standard for
automated signing). Keep it age-sealed at rest in your product's encrypted
secrets repo; decrypt to a chmod-600 tmpfile at cut time and pass its path to
`minisign.Sign`.

## 6. Smoke-test your kit

- `go test ./...` in the library (already green here).
- A `--dry-run` cut of one component that runs the gate and produces a signed,
  checksummed zip you can `minisign -V` and `unzip -l`.
- See `Example_releaseFlow` in `example_test.go` at the repo root for a
  compile-checked worked example of the full compose order.

## Non-goals

release-kit never touches a credential, hostname, or bucket. Distribution,
notarization submission, and catalogs are your product's job.
