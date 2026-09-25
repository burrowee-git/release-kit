package releasekit_test

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

func Example_releaseFlow() {
	ctx := context.Background()

	modules := []vulncheck.Module{
		{Name: "myapp", Dir: "/path/to/myapp"},
	}
	if err := vulncheck.Gate(ctx, modules, vulncheck.GateOpts{
		ReportDir: "/path/to/out/vulncheck-reports",
	}); err != nil {
		fmt.Println("gate failed:", err)
		return
	}

	v, err := version.Stamp(ctx, "/path/to/myapp/VERSION", "/path/to/myapp", version.DateVersionScheme)
	if err != nil {
		fmt.Println("stamp failed:", err)
		return
	}

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

	sums := "/path/to/out/" + v + "/SHA256SUMS"
	if err := checksum.WriteSums(build.Paths(arts), sums); err != nil {
		fmt.Println("checksum failed:", err)
		return
	}

	if err := minisign.Sign(ctx, sums, "/path/to/secrets/minisign.key"); err != nil {
		fmt.Println("minisign failed:", err)
		return
	}

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
