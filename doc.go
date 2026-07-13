// Package releasekit is the root of github.com/burrowee-git/release-kit — a
// brand-agnostic, secret-free set of Go primitives for turning source into
// signed, checksummed, CVE-gated release artifacts. It holds no code itself;
// import the sub-package for the step you need.
//
// release-kit never touches a credential, hostname, or bucket, and never
// performs distribution — where artifacts go (GitHub Releases, R2, a host
// upload, a catalog) is your product's job, done in your own release repo.
//
// The seven sub-packages, roughly in the order a release cut composes them:
//
//   - vulncheck — a fail-closed CVE gate over govulncheck.
//   - version   — compose a version stamp from a semver file + git HEAD sha.
//   - build     — cross-compile binaries for a GOOS/GOARCH matrix, optionally
//     signing darwin outputs.
//   - sign      — code-sign macOS binaries (ad-hoc or a caller-supplied
//     Developer ID identity).
//   - checksum  — write a SHA256SUMS file over a set of artifacts.
//   - minisign  — sign and verify a file with minisign.
//   - pack      — assemble a flat zip archive from a set of files.
//
// See GUIDE.md in the repository root for how to stand up a release-kit for
// a new product, and Example_releaseFlow in this package for the canonical
// compose order.
package releasekit
