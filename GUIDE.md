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
age-sealed in your `.dp` secrets repo — never commit it).

## 3. `cmd/release/main.go` — orchestrate

Call the primitives in order. The CVE gate is MANDATORY on public cuts:

1. parse flags / prompt (`--public-release` = sign + gate; `--vulncheck`; `--apple`);
2. `vulncheck.Gate(modules, opts)` — **first, on every public cut** (fail-closed);
3. `version.Stamp` → `build.Compile` → `checksum.SumFile` → `minisign.Sign` → `pack.Zip`;
4. your distribution (GitHub Release, R2, host upload, catalogs, bootstraps).

## 4. Signing identity is yours

Construct a `sign.Signer` from your config — `sign.AdHocSigner{}` for dev/CI, or
`sign.AppleSigner{Identity: "...", ToolPath: "..."}` for a real Developer ID
release. Nothing about the identity lives in release-kit.

## 5. Minisign key

release-kit expects a **password-less** minisign secret key (standard for
automated signing). Keep it age-sealed at rest in your product's `.dp`; decrypt
to a chmod-600 tmpfile at cut time and pass its path to `minisign.Sign`.

## 6. Smoke-test your kit

- `go test ./...` in the library (already green here).
- A `--dry-run` cut of one component that runs the gate and produces a signed,
  checksummed zip you can `minisign -V` and `unzip -l`.

## Non-goals

release-kit never touches a credential, hostname, or bucket. Distribution,
notarization submission, and catalogs are your product's job.
