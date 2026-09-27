package sign

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

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
	if !strings.HasPrefix(err.Error(), "apple sign:") {
		t.Errorf("Sign error = %q, want wrapped with %q", err.Error(), "apple sign:")
	}
	if !strings.Contains(err.Error(), "stub sign output") {
		t.Errorf("Sign error = %q, want it to carry the tool's output %q", err.Error(), "stub sign output")
	}
}

func argsStubBody(argsFile string) string {
	return "#!/bin/sh\n: > \"" + argsFile + "\"\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> \"" + argsFile + "\"; done\n"
}

func writeArgsStub(t *testing.T, argsFile string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "args-stub")
	if err := os.WriteFile(p, []byte(argsStubBody(argsFile)), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func installArgsStubAs(t *testing.T, name, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(argsStubBody(argsFile)), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func readArgLines(t *testing.T, argsFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestAppleSignerInvokes(t *testing.T) {
	abs, err := filepath.Abs("-x")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		signer func(t *testing.T, argsFile string) AppleSigner
		want   []string
	}{
		{
			name: "plain",
			signer: func(t *testing.T, argsFile string) AppleSigner {
				installArgsStubAs(t, "codesign", argsFile)
				return AppleSigner{Identity: "Developer ID Application: X (TEAM)"}
			},
			want: []string{"--sign", "Developer ID Application: X (TEAM)", "--force", "--options", "runtime", "--timestamp", abs},
		},
		{
			name: "wrapper",
			signer: func(t *testing.T, argsFile string) AppleSigner {
				return AppleSigner{Identity: "ignored", ToolPath: writeArgsStub(t, argsFile)}
			},
			want: []string{"sign", abs},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			argsFile := filepath.Join(t.TempDir(), "args")
			if err := tc.signer(t, argsFile).Sign(context.Background(), "-x"); err != nil {
				t.Fatalf("Sign: %v", err)
			}
			if got := readArgLines(t, argsFile); !slices.Equal(got, tc.want) {
				t.Errorf("signer argv = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAdHocSignerInvokesCodesign(t *testing.T) {
	abs, err := filepath.Abs("-x")
	if err != nil {
		t.Fatal(err)
	}
	argsFile := filepath.Join(t.TempDir(), "args")
	installArgsStubAs(t, "codesign", argsFile)
	if err := (AdHocSigner{}).Sign(context.Background(), "-x"); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	want := []string{"--sign", "-", "--force", abs}
	if got := readArgLines(t, argsFile); !slices.Equal(got, want) {
		t.Errorf("codesign argv = %q, want %q", got, want)
	}
}

func TestAdHocSignerFailureCarriesOutput(t *testing.T) {
	dir := t.TempDir()
	body := "#!/bin/sh\necho 'stub codesign failure'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "codesign"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	err := (AdHocSigner{}).Sign(context.Background(), "bin")
	if err == nil || !strings.Contains(err.Error(), "stub codesign failure") {
		t.Errorf("Sign error = %v, want it to carry the tool's output", err)
	}
}
