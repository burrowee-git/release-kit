package sign

import (
	"context"
	"errors"
	"os/exec"
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
	var ee *exec.ExitError
	_ = errors.As(err, &ee) // just assert non-nil above; type is best-effort
}
