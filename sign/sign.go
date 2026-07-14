// Package sign code-signs macOS binaries with a caller-supplied identity. It is
// identity-agnostic: no signing account, team, or tool is hardcoded.
package sign

import (
	"context"
	"fmt"
	"os/exec"
)

// Signer signs a single binary in place.
type Signer interface {
	Sign(ctx context.Context, binaryPath string) error
}

// AdHocSigner applies an ad-hoc signature (`codesign --sign -`). macOS needs any
// signature to exec a native binary; this is the default for dev/CI builds.
type AdHocSigner struct{}

func (AdHocSigner) Sign(ctx context.Context, binaryPath string) error {
	cmd := exec.CommandContext(ctx, "codesign", "--sign", "-", "--force", binaryPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adhoc codesign: %w\n%s", err, out)
	}
	return nil
}

// AppleSigner applies a real Developer ID signature. With ToolPath empty it
// drives `codesign` directly using Identity; with ToolPath set it delegates to a
// wrapper invoked as `<ToolPath> sign <path>` (e.g. a product's signing helper).
// Notarization is available via Notarizer (v0.1.1); App Store upload is not this library's job.
type AppleSigner struct {
	Identity string
	ToolPath string
}

func (a AppleSigner) command(binaryPath string) (string, []string) {
	if a.ToolPath != "" {
		return a.ToolPath, []string{"sign", binaryPath}
	}
	return "codesign", []string{"--sign", a.Identity, "--force", "--options", "runtime", "--timestamp", binaryPath}
}

func (a AppleSigner) Sign(ctx context.Context, binaryPath string) error {
	bin, args := a.command(binaryPath)
	if out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("apple sign: %w\n%s", err, out)
	}
	return nil
}
