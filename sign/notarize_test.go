package sign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotarizerCommand(t *testing.T) {
	n := Notarizer{ToolPath: "modernech-sign"}
	bin, args := n.command("/tmp/burrowee-cli-darwin-arm64.zip")
	if bin != "modernech-sign" {
		t.Fatalf("bin = %q, want modernech-sign", bin)
	}
	want := []string{"notarize", "/tmp/burrowee-cli-darwin-arm64.zip"}
	if len(args) != len(want) || args[0] != want[0] || args[1] != want[1] {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestNotarizerRequiresToolPath(t *testing.T) {
	// Empty ToolPath is a usage error — burrowee always sets it to modernech-sign.
	n := Notarizer{}
	err := n.Notarize(context.Background(), "/tmp/x.zip")
	if err == nil {
		t.Fatal("expected error for empty ToolPath")
	}
}

func TestNotarizerSurfacesToolError(t *testing.T) {
	n := Notarizer{ToolPath: "/nonexistent/modernech-sign-xyz"}
	err := n.Notarize(context.Background(), "/tmp/x.zip")
	if err == nil {
		t.Fatal("expected error when tool missing")
	}
}

func TestNotarizeAbsolutizesPath(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	n := Notarizer{ToolPath: writeArgsStub(t, argsFile)}
	// A path beginning with "-" must not reach the tool as a bare flag-shaped arg.
	if err := n.Notarize(context.Background(), "-x"); err != nil {
		t.Fatalf("Notarize: %v", err)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 { // "notarize", <path>
		t.Fatalf("stub saw args %v, want [notarize <path>]", lines)
	}
	if strings.HasPrefix(lines[1], "-") || !filepath.IsAbs(lines[1]) {
		t.Errorf("path arg %q not absolutized", lines[1])
	}
}
