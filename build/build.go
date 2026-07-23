// Package build cross-compiles Go binaries with pinned-tag discipline (GOWORK
// controllable per binary) and optional macOS signing. It is product-agnostic:
// the caller supplies the binary→package list and fully-computed ldflags.
package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/burrowee-git/release-kit/sign"
)

// Target is one GOOS/GOARCH pair to build for.
type Target struct{ OS, Arch string }

// BinSpec is one binary to build. SubDir (relative to SrcDir) selects a nested
// module to build from; GoWork "off" forces module (non-workspace) mode so
// pinned tags resolve reproducibly. GoWork left empty defaults to "off" —
// workspace mode is intentionally not the default, since it can silently
// resolve a different module graph than vulncheck.Gate scanned.
type BinSpec struct {
	Name    string
	Package string
	Ldflags string
	SubDir  string
	GoWork  string
}

// Spec configures a Compile run: source tree, output layout, cross-compile
// targets, binaries to build, and an optional Signer for darwin outputs.
type Spec struct {
	SrcDir  string
	GoBin   string
	OutDir  string
	Targets []Target
	Bins    []BinSpec
	Signer  sign.Signer
}

// Artifact is one built binary's location and build metadata.
type Artifact struct {
	Bin, OS, Arch, Path string
	// Signed is true when Compile actually code-signed this binary (a darwin
	// target built on a darwin host with a non-nil Signer).
	Signed bool
}

// Paths returns each Artifact's Path, in order — convenience glue for feeding
// checksum.WriteSums or any other API that wants a flat file-path list. pack.Zip
// wants []Content (name + source path), not a bare []string, so it doesn't
// consume Paths directly.
func Paths(arts []Artifact) []string {
	out := make([]string, len(arts))
	for i, a := range arts {
		out[i] = a.Path
	}
	return out
}

// Compile builds every Bin for every Target into OutDir/<os>-<arch>/<Name>.
// darwin outputs are signed with Signer when the build host is darwin and Signer
// is non-nil (macOS refuses to exec an unsigned native binary). When the build
// host is not darwin, darwin outputs are left unsigned, since codesign is
// macOS-only.
func Compile(ctx context.Context, spec Spec) ([]Artifact, error) {
	goBin := spec.GoBin
	if goBin == "" {
		goBin = "go"
	}
	// Resolve OutDir to one absolute base up front: MkdirAll runs in the process
	// cwd while `go build -o` runs with cmd.Dir=buildDir, so a relative OutDir
	// would otherwise be resolved against two different roots.
	outBase, err := filepath.Abs(spec.OutDir)
	if err != nil {
		return nil, fmt.Errorf("build: resolve OutDir %q: %w", spec.OutDir, err)
	}
	var arts []Artifact
	host := runtime.GOOS
	for _, tgt := range spec.Targets {
		outDir := filepath.Join(outBase, tgt.OS+"-"+tgt.Arch)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return nil, err
		}
		for _, b := range spec.Bins {
			outPath := filepath.Join(outDir, b.Name)
			buildDir := spec.SrcDir
			if b.SubDir != "" {
				buildDir = filepath.Join(spec.SrcDir, b.SubDir)
			}
			goWork := b.GoWork
			if goWork == "" {
				goWork = "off"
			}
			cmd := exec.CommandContext(ctx, goBin, "build", "-trimpath", "-ldflags", b.Ldflags, "-o", outPath, b.Package)
			cmd.Dir = buildDir
			cmd.Env = append(os.Environ(),
				"CGO_ENABLED=0", "GOOS="+tgt.OS, "GOARCH="+tgt.Arch, "GOWORK="+goWork)
			if out, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("build %s (%s/%s): %w\n%s", b.Name, tgt.OS, tgt.Arch, err, out)
			}
			signed := false
			if tgt.OS == "darwin" && host == "darwin" && spec.Signer != nil {
				if err := spec.Signer.Sign(ctx, outPath); err != nil {
					return nil, fmt.Errorf("sign %s: %w", outPath, err)
				}
				signed = true
			}
			arts = append(arts, Artifact{Bin: b.Name, OS: tgt.OS, Arch: tgt.Arch, Path: outPath, Signed: signed})
		}
	}
	return arts, nil
}
