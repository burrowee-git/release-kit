package sign

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNotarizerRequiresToolPath(t *testing.T) {
	n := Notarizer{}
	err := n.Notarize(context.Background(), "/tmp/x.zip")
	if err == nil || !strings.Contains(err.Error(), "ToolPath is required") {
		t.Fatalf("Notarize error = %v, want the ToolPath usage error", err)
	}
}

func TestNotarizerSurfacesToolError(t *testing.T) {
	n := Notarizer{ToolPath: writeStub(t, 1)}
	err := n.Notarize(context.Background(), "/tmp/x.zip")
	if err == nil || !strings.Contains(err.Error(), "stub sign output") {
		t.Fatalf("Notarize error = %v, want it to carry the tool's output", err)
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
