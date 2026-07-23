package minisign

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSignVerifyRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("minisign"); err != nil {
		t.Skip("minisign not installed")
	}
	dir := t.TempDir()
	sec := filepath.Join(dir, "key.sec")
	pub := filepath.Join(dir, "key.pub")
	// -W → password-less key (no interactive prompt)
	if out, err := exec.Command("minisign", "-G", "-W", "-p", pub, "-s", sec).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v\n%s", err, out)
	}
	sums := filepath.Join(dir, "SHA256SUMS.txt")
	if err := os.WriteFile(sums, []byte("deadbeef  x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Sign(context.Background(), sums, sec); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := os.Stat(sums + ".minisig"); err != nil {
		t.Fatalf("no .minisig written: %v", err)
	}
	if err := Verify(context.Background(), sums, pub); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	// tamper → verify must fail
	if err := os.WriteFile(sums, []byte("tampered  x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Verify(context.Background(), sums, pub); err == nil {
		t.Fatal("Verify passed a tampered file")
	}
}

func TestSignAbsolutizesPaths(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	stubDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(stubDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\n: > \"" + argsFile + "\"\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> \"" + argsFile + "\"; done\n"
	if err := os.WriteFile(filepath.Join(stubDir, "minisign"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Caller-supplied dash-prefixed paths must not reach minisign as flags.
	if err := Sign(context.Background(), "-m-sums", "-s-key"); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	lines := readArgs(t, argsFile)
	if len(lines) != 5 { // minisign -S -s <key> -m <sums>
		t.Fatalf("stub saw args %v", lines)
	}
	if !filepath.IsAbs(lines[2]) {
		t.Errorf("secretKeyPath arg %q not absolutized", lines[2])
	}
	if !filepath.IsAbs(lines[4]) {
		t.Errorf("sumsFile arg %q not absolutized", lines[4])
	}
}

func readArgs(t *testing.T, argsFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}
