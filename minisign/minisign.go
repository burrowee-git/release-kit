// Package minisign signs and verifies files with minisign (Frank Denis'
// signature tool). It expects a PASSWORD-LESS secret key (the standard shape for
// automated release signing; the key stays age-sealed at rest in the product's
// secrets repo). No password is ever prompted or handled here.
package minisign

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Sign writes sumsFile+".minisig" by signing sumsFile with the password-less
// minisign secret key at secretKeyPath.
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

// Verify checks sumsFile against sumsFile+".minisig" using the public key file.
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
