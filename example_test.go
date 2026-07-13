package releasekit_test

// This example is illustrative and compile-checked only — it has no "Output:"
// comment, so `go test` compiles it (pinning the sub-package signatures
// against drift, and rendering on pkg.go.dev) but does not execute it. A real
// run would need a real module tree, govulncheck, minisign, and (for
// AppleSigner) codesign/an identity.

import (
	"context"
	"fmt"

	"github.com/burrowee-git/release-kit/build"
	"github.com/burrowee-git/release-kit/checksum"
	"github.com/burrowee-git/release-kit/minisign"
	"github.com/burrowee-git/release-kit/pack"
	"github.com/burrowee-git/release-kit/sign"
	"github.com/burrowee-git/release-kit/version"
	"github.com/burrowee-git/release-kit/vulncheck"
)

// Example_releaseFlow shows the canonical compose order for a release cut:
// the CVE gate first, then stamp, build, checksum, minisign, and pack. Paths
// are illustrative — swap in your product's real module list, source tree,
// and signing identity.
func Example_releaseFlow() {
	ctx := context.Background()

	// 1. The CVE gate runs first, on every public cut, and is fail-closed.
	modules := []vulncheck.Module{
		{Name: "myapp", Dir: "/path/to/myapp"},
	}
	if err := vulncheck.Gate(ctx, modules, vulncheck.GateOpts{
		ReportDir: "/path/to/out/vulncheck-reports",
	}); err != nil {
		fmt.Println("gate failed:", err)
		return
	}

	// 2. Stamp the version from a semver file + the source tree's HEAD sha.
	v, err := version.Stamp(ctx, "/path/to/myapp/VERSION", "/path/to/myapp", version.DateVersionScheme)
	if err != nil {
		fmt.Println("stamp failed:", err)
		return
	}

	// 3. Cross-compile the binary matrix, signing darwin outputs.
	arts, err := build.Compile(ctx, build.Spec{
		SrcDir: "/path/to/myapp",
		OutDir: "/path/to/out/" + v,
		Targets: []build.Target{
			{OS: "darwin", Arch: "arm64"},
			{OS: "linux", Arch: "amd64"},
		},
		Bins: []build.BinSpec{
			{Name: "myapp", Package: "./cmd/myapp", Ldflags: "-X main.version=" + v},
		},
		Signer: sign.AdHocSigner{},
	})
	if err != nil {
		fmt.Println("compile failed:", err)
		return
	}

	// 4. checksum.WriteSums wants the artifacts' file paths; build.Paths does
	// the []Artifact -> []string glue.
	sums := "/path/to/out/" + v + "/SHA256SUMS"
	if err := checksum.WriteSums(build.Paths(arts), sums); err != nil {
		fmt.Println("checksum failed:", err)
		return
	}

	// 5. Sign the checksums file with the product's minisign key.
	if err := minisign.Sign(ctx, sums, "/path/to/secrets/minisign.key"); err != nil {
		fmt.Println("minisign failed:", err)
		return
	}

	// 6. Pack the artifacts + checksums + signature into a distributable zip.
	// pack.Zip wants []Content (name + source path), so the artifacts are
	// re-listed here rather than reusing build.Paths' []string.
	contents := make([]pack.Content, 0, len(arts)+2)
	for _, a := range arts {
		contents = append(contents, pack.Content{Src: a.Path})
	}
	contents = append(contents,
		pack.Content{Src: sums},
		pack.Content{Src: sums + ".minisig"},
	)
	if err := pack.Zip(pack.Spec{
		Contents: contents,
		Out:      "/path/to/out/" + v + ".zip",
	}); err != nil {
		fmt.Println("pack failed:", err)
		return
	}

	fmt.Println("release cut complete")
}
