// Package build cross-compiles Go binaries with pinned-tag discipline (GOWORK
// controllable per binary) and optional macOS signing. It is product-agnostic:
// the caller supplies the binary→package list and fully-computed ldflags.
package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/burrowee-git/release-kit/sign"
)

type Target struct{ OS, Arch string }

// BinSpec is one binary to build. SubDir (relative to SrcDir) selects a nested
// module to build from; GoWork "off" forces module (non-workspace) mode so pinned
// tags resolve reproducibly.
type BinSpec struct {
	Name    string
	Package string
	Ldflags string
	SubDir  string
	GoWork  string
}

type Spec struct {
	SrcDir  string
	GoBin   string
	OutDir  string
	Targets []Target
	Bins    []BinSpec
	Signer  sign.Signer
}

type Artifact struct {
	Bin, OS, Arch, Path string
}

// Compile builds every Bin for every Target into OutDir/<os>-<arch>/<Name>.
// darwin outputs are signed with Signer when the build host is darwin and Signer
// is non-nil (macOS refuses to exec an unsigned native binary).
func Compile(spec Spec) ([]Artifact, error) {
	goBin := spec.GoBin
	if goBin == "" {
		goBin = "go"
	}
	var arts []Artifact
	host := runtime.GOOS
	for _, tgt := range spec.Targets {
		outDir := filepath.Join(spec.OutDir, tgt.OS+"-"+tgt.Arch)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return nil, err
		}
		for _, b := range spec.Bins {
			outPath := filepath.Join(outDir, b.Name)
			buildDir := spec.SrcDir
			if b.SubDir != "" {
				buildDir = filepath.Join(spec.SrcDir, b.SubDir)
			}
			cmd := exec.Command(goBin, "build", "-trimpath", "-ldflags", b.Ldflags, "-o", outPath, b.Package)
			cmd.Dir = buildDir
			cmd.Env = append(os.Environ(),
				"CGO_ENABLED=0", "GOOS="+tgt.OS, "GOARCH="+tgt.Arch, "GOWORK="+b.GoWork)
			if out, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("build %s (%s/%s): %w\n%s", b.Name, tgt.OS, tgt.Arch, err, out)
			}
			if tgt.OS == "darwin" && host == "darwin" && spec.Signer != nil {
				if err := spec.Signer.Sign(outPath); err != nil {
					return nil, fmt.Errorf("sign %s: %w", outPath, err)
				}
			}
			arts = append(arts, Artifact{Bin: b.Name, OS: tgt.OS, Arch: tgt.Arch, Path: outPath})
		}
	}
	return arts, nil
}
