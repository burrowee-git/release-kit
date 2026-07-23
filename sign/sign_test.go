package sign

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
	bin, args = AppleSigner{Identity: "ignored", ToolPath: "signtool"}.command("/tmp/b")
	got = bin + " " + strings.Join(args, " ")
	if got != "signtool sign /tmp/b" {
		t.Errorf("wrapper: got=%q", got)
	}
}

func writeStub(t *testing.T, exit int) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "signer-stub")
	body := "#!/bin/sh\necho \"stub sign output\"\nexit " + strconv.Itoa(exit) + "\n"
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAppleSignerSignSuccess(t *testing.T) {
	s := AppleSigner{ToolPath: writeStub(t, 0)}
	if err := s.Sign(context.Background(), "/tmp/b"); err != nil {
		t.Fatalf("Sign: %v", err)
	}
}

func TestAppleSignerSignError(t *testing.T) {
	s := AppleSigner{ToolPath: writeStub(t, 1)}
	err := s.Sign(context.Background(), "/tmp/b")
	if err == nil {
		t.Fatal("Sign: want error on non-zero exit, got nil")
	}
	if !strings.Contains(err.Error(), "apple sign:") {
		t.Errorf("Sign error = %q, want wrapped with %q", err.Error(), "apple sign:")
	}
}

// writeArgsStub writes an executable that records the args it receives, one per
// line, to argsFile. Shared by the sign + notarize absolutization tests.
func writeArgsStub(t *testing.T, argsFile string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "args-stub")
	body := "#!/bin/sh\n: > \"" + argsFile + "\"\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> \"" + argsFile + "\"; done\n"
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAppleSignerAbsolutizesPath(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	s := AppleSigner{ToolPath: writeArgsStub(t, argsFile)}
	// A path beginning with "-" must not reach the tool as a bare flag-shaped arg.
	if err := s.Sign(context.Background(), "-x"); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 { // "sign", <path>
		t.Fatalf("stub saw args %v, want [sign <path>]", lines)
	}
	if strings.HasPrefix(lines[1], "-") || !filepath.IsAbs(lines[1]) {
		t.Errorf("path arg %q not absolutized", lines[1])
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
	build := exec.Command("go", "build", "-o", binp, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	if err := (AdHocSigner{}).Sign(context.Background(), binp); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if out, err := exec.Command("codesign", "-v", binp).CombinedOutput(); err != nil {
		t.Fatalf("codesign -v failed: %v\n%s", err, out)
	}
}
