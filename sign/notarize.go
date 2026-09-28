package sign

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

type Notarizer struct {
	ToolPath string
}

func (n Notarizer) command(artifactPath string) (string, []string) {
	return n.ToolPath, []string{"notarize", artifactPath}
}

func (n Notarizer) Notarize(ctx context.Context, artifactPath string) error {
	if n.ToolPath == "" {
		return fmt.Errorf("notarize: ToolPath is required")
	}
	artifactPath, err := filepath.Abs(artifactPath)
	if err != nil {
		return fmt.Errorf("notarize: %w", err)
	}
	bin, args := n.command(artifactPath)
	if out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("notarize: %w\n%s", err, out)
	}
	return nil
}
