# release-kit

Brand-agnostic, secret-free Go primitives for cutting signed, checksummed,
CVE-gated software releases. Turn source into verifiable release artifacts;
distribution stays in your own release repo.

Packages: `version` · `build` · `sign` · `checksum` · `minisign` · `pack` · `vulncheck`.

See [GUIDE.md](GUIDE.md) to stand up a release-kit for a new product.

No third-party dependencies. Shells out only to `go`, `codesign`, `minisign`,
and `govulncheck`.
