package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/burrowee-git/release-kit/sign"
)

func writeTinyModule(t *testing.T, mainSrc string) string {
	t.Helper()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module tiny\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	return src
}

func TestCompileHostBinaryWithLdflags(t *testing.T) {
	src := writeTinyModule(t, "package main\nimport \"fmt\"\nvar version = \"dev\"\nfunc main(){ fmt.Print(version) }\n")
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

type refusingSigner struct{ t *testing.T }

func (r refusingSigner) Sign(ctx context.Context, binaryPath string) error {
	r.t.Helper()
	r.t.Fatal("Sign must not be called for a foreign-OS build")
	return nil
}

func TestCompileForeignOSNotSigned(t *testing.T) {
	src := writeTinyModule(t, "package main\nfunc main(){}\n")
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

func TestCompileRelativeOutDirResolvesToOneBase(t *testing.T) {
	src := writeTinyModule(t, "package main\nfunc main(){}\n")
	work := t.TempDir()
	t.Chdir(work)

	arts, err := Compile(context.Background(), Spec{
		SrcDir: src, GoBin: "go", OutDir: "dist",
		Targets: []Target{{OS: runtime.GOOS, Arch: runtime.GOARCH}},
		Bins:    []BinSpec{{Name: "tiny", Package: "."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("want 1 artifact, got %d", len(arts))
	}
	got := arts[0].Path
	if !filepath.IsAbs(got) {
		t.Errorf("Artifact.Path = %q, want absolute", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("binary not at recorded path %q: %v", got, err)
	}
	want := filepath.Join(work, "dist", runtime.GOOS+"-"+runtime.GOARCH, "tiny")
	if got != want {
		t.Errorf("Path=%q want %q", got, want)
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

func TestCompileGoWorkReachesTheBuild(t *testing.T) {
	cases := []struct {
		name   string
		goWork string
		want   string
	}{
		{name: "empty_defaults_to_off", goWork: "", want: "off"},
		{name: "explicit_value_passes_through", goWork: "/src/go.work", want: "/src/go.work"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := filepath.Join(t.TempDir(), "gowork")
			goBin := filepath.Join(t.TempDir(), "go-recorder")
			body := "#!/bin/sh\nprintf '%s' \"$GOWORK\" > \"" + record + "\"\n"
			if err := os.WriteFile(goBin, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			_, err := Compile(context.Background(), Spec{
				SrcDir: t.TempDir(), GoBin: goBin, OutDir: t.TempDir(),
				Targets: []Target{{OS: "linux", Arch: "amd64"}},
				Bins:    []BinSpec{{Name: "tiny", Package: ".", GoWork: tc.goWork}},
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("GOWORK=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompileRunsGoBuild(t *testing.T) {
	src := t.TempDir()
	sub := filepath.Join(src, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		subDir  string
		exit    int
		wantDir string
		wantErr string
	}{
		{name: "sub_dir", subDir: "nested", wantDir: sub},
		{name: "build_failure", exit: 1, wantDir: src, wantErr: "stub build failure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := filepath.Join(t.TempDir(), "pwd")
			goBin := filepath.Join(t.TempDir(), "go-recorder")
			body := "#!/bin/sh\npwd -P > \"" + record + "\"\necho 'stub build failure'\nexit " + strconv.Itoa(tc.exit) + "\n"
			if err := os.WriteFile(goBin, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			_, err := Compile(context.Background(), Spec{
				SrcDir: src, GoBin: goBin, OutDir: t.TempDir(),
				Targets: []Target{{OS: "linux", Arch: "amd64"}},
				Bins:    []BinSpec{{Name: "tiny", Package: ".", SubDir: tc.subDir}},
			})
			if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("Compile error = %v, want one containing %q", err, tc.wantErr)
			}
			got, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			wantDir, err := filepath.EvalSymlinks(tc.wantDir)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(got)) != wantDir {
				t.Errorf("go build ran in %q, want %q", got, wantDir)
			}
		})
	}
}
