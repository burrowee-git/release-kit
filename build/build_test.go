package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/burrowee-git/release-kit/sign"
)

func TestCompileHostBinaryWithLdflags(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module tiny\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "main.go"), []byte(
		"package main\nimport \"fmt\"\nvar version = \"dev\"\nfunc main(){ fmt.Print(version) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	wantSigned := runtime.GOOS == "darwin"
	if arts[0].Signed != wantSigned {
		t.Errorf("Signed=%v, want %v (host %s)", arts[0].Signed, wantSigned, runtime.GOOS)
	}
}

// refusingSigner fails the test the moment Sign is invoked, proving a
// foreign-OS build never reaches the signing step.
type refusingSigner struct{ t *testing.T }

func (r refusingSigner) Sign(ctx context.Context, binaryPath string) error {
	r.t.Helper()
	r.t.Fatal("Sign must not be called for a foreign-OS build")
	return nil
}

func TestCompileForeignOSNotSigned(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module tiny\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "main.go"), []byte(
		"package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()

	foreignOS := "linux"
	if runtime.GOOS == "linux" {
		foreignOS = "darwin"
	}

	arts, err := Compile(context.Background(), Spec{
		SrcDir: src, GoBin: "go", OutDir: out,
		Targets: []Target{{OS: foreignOS, Arch: "amd64"}},
		Bins:    []BinSpec{{Name: "tiny", Package: "."}},
		Signer:  refusingSigner{t: t},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("want 1 artifact, got %d", len(arts))
	}
	if arts[0].Signed {
		t.Error("Signed=true for a foreign-OS build")
	}
}

func TestPaths(t *testing.T) {
	arts := []Artifact{{Path: "a"}, {Path: "b"}}
	got := Paths(arts)
	want := []string{"a", "b"}
	if !slices.Equal(got, want) {
		t.Errorf("Paths=%v want %v", got, want)
	}
}
