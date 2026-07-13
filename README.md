# release-kit

Brand-agnostic, secret-free Go primitives for cutting signed, checksummed,
CVE-gated software releases. Turn source into verifiable release artifacts;
distribution stays in your own release repo.

Packages: `version` · `build` · `sign` · `checksum` · `minisign` · `pack` · `vulncheck`.

See [GUIDE.md](GUIDE.md) to stand up a release-kit for a new product, or
`Example_releaseFlow` in `example_test.go` (also rendered on
[pkg.go.dev](https://pkg.go.dev/github.com/burrowee-git/release-kit)) for the
compose order.

No third-party dependencies. Shells out only to `go`, `git`, `codesign`,
`minisign`, and `govulncheck`.
