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

type Target struct{ OS, Arch string }

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
	Signed              bool
}

func Paths(arts []Artifact) []string {
	out := make([]string, len(arts))
	for i, a := range arts {
		out[i] = a.Path
	}
	return out
}

func Compile(ctx context.Context, spec Spec) ([]Artifact, error) {
	goBin := spec.GoBin
	if goBin == "" {
		goBin = "go"
	}
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
