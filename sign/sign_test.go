package sign

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAppleSignerCommand(t *testing.T) {
	// plain codesign mode (no wrapper)
	bin, args := AppleSigner{Identity: "Developer ID Application: X (TEAM)"}.command("/tmp/b")
	got := bin + " " + strings.Join(args, " ")
	want := "codesign --sign Developer ID Application: X (TEAM) --force --options runtime --timestamp /tmp/b"
	if got != want {
		t.Errorf("plain:\n got=%q\nwant=%q", got, want)
	}
	// wrapper mode
	bin, args = AppleSigner{Identity: "ignored", ToolPath: "modernech-sign"}.command("/tmp/b")
	got = bin + " " + strings.Join(args, " ")
	if got != "modernech-sign sign /tmp/b" {
		t.Errorf("wrapper: got=%q", got)
	}
}

func TestAdHocSignerRunsOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("ad-hoc codesign is darwin-only")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	os.WriteFile(src, []byte("package main\nfunc main(){}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tiny\ngo 1.25.0\n"), 0o644)
	binp := filepath.Join(dir, "tiny")
	build := exec.Command("/opt/homebrew/bin/go", "build", "-o", binp, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	if err := (AdHocSigner{}).Sign(binp); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if out, err := exec.Command("codesign", "-v", binp).CombinedOutput(); err != nil {
		t.Fatalf("codesign -v failed: %v\n%s", err, out)
	}
}
