package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/burrowee-git/release-kit/sign"
)

func TestCompileHostBinaryWithLdflags(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "go.mod"), []byte("module tiny\ngo 1.25.0\n"), 0o644)
	os.WriteFile(filepath.Join(src, "main.go"), []byte(
		"package main\nimport \"fmt\"\nvar version = \"dev\"\nfunc main(){ fmt.Print(version) }\n"), 0o644)
	out := t.TempDir()

	arts, err := Compile(context.Background(), Spec{
		SrcDir: src, GoBin: "go", OutDir: out,
		Targets: []Target{{OS: runtime.GOOS, Arch: runtime.GOARCH}},
		Bins:    []BinSpec{{Name: "tiny", Package: ".", Ldflags: "-X main.version=STAMP123"}},
		Signer:  sign.AdHocSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("want 1 artifact, got %d", len(arts))
	}
	want := filepath.Join(out, runtime.GOOS+"-"+runtime.GOARCH, "tiny")
	if arts[0].Path != want {
		t.Errorf("path=%q want %q", arts[0].Path, want)
	}
	got, err := exec.Command(want).Output()
	if err != nil {
		t.Fatalf("run built binary: %v", err)
	}
	if strings.TrimSpace(string(got)) != "STAMP123" {
		t.Errorf("ldflags not applied: binary printed %q", got)
	}
}
