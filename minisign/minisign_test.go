package minisign

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestSignVerifyRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("minisign"); err != nil {
		t.Skip("minisign not on PATH; owner: release-kit maintainers; re-enable: install minisign where the suite runs (masdetta-ci has /usr/bin/minisign)")
	}
	dir := t.TempDir()
	sec := filepath.Join(dir, "key.sec")
	pub := filepath.Join(dir, "key.pub")
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
	if err := os.WriteFile(sums, []byte("tampered  x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Verify(context.Background(), sums, pub); err == nil {
		t.Fatal("Verify passed a tampered file")
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

func installStub(t *testing.T, exit int, output string) string {
	t.Helper()
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	stubDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(stubDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\n: > \"" + argsFile + "\"\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> \"" + argsFile + "\"; done\n" +
		"printf '%s\\n' '" + output + "'\nexit " + strconv.Itoa(exit) + "\n"
	if err := os.WriteFile(filepath.Join(stubDir, "minisign"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func absPath(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestArgv(t *testing.T) {
	cases := []struct {
		name string
		call func() error
		want []string
	}{
		{
			name: "sign",
			call: func() error { return Sign(context.Background(), "-m-sums", "-s-key") },
			want: []string{"-S", "-s", absPath(t, "-s-key"), "-m", absPath(t, "-m-sums")},
		},
		{
			name: "verify",
			call: func() error { return Verify(context.Background(), "-m-sums", "-p-pub") },
			want: []string{"-V", "-p", absPath(t, "-p-pub"), "-m", absPath(t, "-m-sums")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			argsFile := installStub(t, 0, "")
			if err := tc.call(); err != nil {
				t.Fatalf("call: %v", err)
			}
			if got := readArgs(t, argsFile); !slices.Equal(got, tc.want) {
				t.Errorf("minisign argv = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestToolFailureCarriesOutput(t *testing.T) {
	cases := []struct {
		name string
		call func() error
	}{
		{name: "sign", call: func() error { return Sign(context.Background(), "sums", "key") }},
		{name: "verify", call: func() error { return Verify(context.Background(), "sums", "pub") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installStub(t, 1, "stub minisign failure")
			if err := tc.call(); err == nil || !strings.Contains(err.Error(), "stub minisign failure") {
				t.Errorf("error = %v, want one carrying the tool's output", err)
			}
		})
	}
}
