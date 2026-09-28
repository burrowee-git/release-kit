package minisign

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

func Sign(ctx context.Context, sumsFile, secretKeyPath string) error {
	sumsFile, err := filepath.Abs(sumsFile)
	if err != nil {
		return fmt.Errorf("minisign sign: %w", err)
	}
	secretKeyPath, err = filepath.Abs(secretKeyPath)
	if err != nil {
		return fmt.Errorf("minisign sign: %w", err)
	}
	cmd := exec.CommandContext(ctx, "minisign", "-S", "-s", secretKeyPath, "-m", sumsFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("minisign sign: %w\n%s", err, out)
	}
	return nil
}

func Verify(ctx context.Context, sumsFile, pubKeyPath string) error {
	sumsFile, err := filepath.Abs(sumsFile)
	if err != nil {
		return fmt.Errorf("minisign verify: %w", err)
	}
	pubKeyPath, err = filepath.Abs(pubKeyPath)
	if err != nil {
		return fmt.Errorf("minisign verify: %w", err)
	}
	cmd := exec.CommandContext(ctx, "minisign", "-V", "-p", pubKeyPath, "-m", sumsFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("minisign verify: %w\n%s", err, out)
	}
	return nil
}
