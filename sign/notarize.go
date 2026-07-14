package sign

import (
	"context"
	"fmt"
	"os/exec"
)

// Notarizer submits a built, signed macOS artifact (a zip or binary) for Apple
// notarization by delegating to a product's signing helper invoked as
// `<ToolPath> notarize <path>` (e.g. modernech-sign). It is deliberately narrow:
// it notarizes only. Stapling is the caller's concern (bare-binary zips can't be
// stapled — the ticket lives in Apple's online DB), and App Store / TestFlight
// upload is NOT this library's job.
type Notarizer struct {
	ToolPath string
}

func (n Notarizer) command(artifactPath string) (string, []string) {
	return n.ToolPath, []string{"notarize", artifactPath}
}

// Notarize submits artifactPath for notarization. ToolPath must be set.
func (n Notarizer) Notarize(ctx context.Context, artifactPath string) error {
	if n.ToolPath == "" {
		return fmt.Errorf("notarize: ToolPath is required")
	}
	bin, args := n.command(artifactPath)
	if out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("notarize: %w\n%s", err, out)
	}
	return nil
}
