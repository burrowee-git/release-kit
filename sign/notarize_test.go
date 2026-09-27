package sign

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNotarizerCommand(t *testing.T) {
	n := Notarizer{ToolPath: "apple-sign"}
	bin, args := n.command("/tmp/example-cli-darwin-arm64.zip")
	if bin != "apple-sign" {
		t.Fatalf("bin = %q, want apple-sign", bin)
	}
	want := []string{"notarize", "/tmp/example-cli-darwin-arm64.zip"}
	if len(args) != len(want) || args[0] != want[0] || args[1] != want[1] {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestNotarizerRequiresToolPath(t *testing.T) {
	n := Notarizer{}
	err := n.Notarize(context.Background(), "/tmp/x.zip")
	if err == nil || !strings.Contains(err.Error(), "ToolPath is required") {
		t.Fatalf("Notarize error = %v, want the ToolPath usage error", err)
	}
}

func TestNotarizerSurfacesToolError(t *testing.T) {
	n := Notarizer{ToolPath: "/nonexistent/apple-sign-xyz"}
	err := n.Notarize(context.Background(), "/tmp/x.zip")
	if err == nil {
		t.Fatal("expected error when tool missing")
	}
}

func TestNotarizeAbsolutizesPath(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	n := Notarizer{ToolPath: writeArgsStub(t, argsFile)}
	if err := n.Notarize(context.Background(), "-x"); err != nil {
		t.Fatalf("Notarize: %v", err)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs("-x")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"notarize", abs}
	if got := strings.Split(strings.TrimSpace(string(data)), "\n"); !slices.Equal(got, want) {
		t.Errorf("notarize argv = %q, want %q", got, want)
	}
}
