package sign

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

type Signer interface {
	Sign(ctx context.Context, binaryPath string) error
}

type AdHocSigner struct{}

func (AdHocSigner) Sign(ctx context.Context, binaryPath string) error {
	binaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("adhoc codesign: %w", err)
	}
	cmd := exec.CommandContext(ctx, "codesign", "--sign", "-", "--force", binaryPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adhoc codesign: %w\n%s", err, out)
	}
	return nil
}

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
	binaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("apple sign: %w", err)
	}
	bin, args := a.command(binaryPath)
	if out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("apple sign: %w\n%s", err, out)
	}
	return nil
}
