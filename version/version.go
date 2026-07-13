// Package version composes release version stamps from a semver source-of-truth
// file and a source worktree's HEAD sha. The stamp format is a caller-supplied
// Scheme, so each product picks its own layout.
package version

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type BumpKind int

const (
	BumpPatch BumpKind = iota
	BumpMinor
	BumpMajor
)

// Scheme formats a full stamp from its parts. dateUTC is pre-formatted YYYY.MM.DD.
type Scheme func(semver, sha, dateUTC string) string

// DateVersionScheme yields "v<semver>.<YYYY.MM.DD>.<sha>".
func DateVersionScheme(semver, sha, dateUTC string) string {
	return "v" + semver + "." + dateUTC + "." + sha
}

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Bump returns the next "X.Y.Z" for cur given kind.
func Bump(cur string, kind BumpKind) (string, error) {
	if !semverRe.MatchString(cur) {
		return "", fmt.Errorf("not MAJOR.MINOR.PATCH: %q", cur)
	}
	p := strings.SplitN(cur, ".", 3)
	maj, _ := strconv.Atoi(p[0])
	min, _ := strconv.Atoi(p[1])
	pat, _ := strconv.Atoi(p[2])
	switch kind {
	case BumpPatch:
		pat++
	case BumpMinor:
		min, pat = min+1, 0
	case BumpMajor:
		maj, min, pat = maj+1, 0, 0
	default:
		return "", fmt.Errorf("unknown bump kind %d", kind)
	}
	return fmt.Sprintf("%d.%d.%d", maj, min, pat), nil
}

// Stamp reads the semver from semverFile, the short-8 HEAD sha of srcDir, today's
// UTC date, and applies scheme. ctx bounds the git subprocess.
func Stamp(ctx context.Context, semverFile, srcDir string, scheme Scheme) (string, error) {
	raw, err := os.ReadFile(semverFile)
	if err != nil {
		return "", err
	}
	semver := strings.TrimSpace(string(raw))
	if !semverRe.MatchString(semver) {
		return "", fmt.Errorf("%s: not MAJOR.MINOR.PATCH: %q", semverFile, semver)
	}
	out, err := exec.CommandContext(ctx, "git", "-C", srcDir, "rev-parse", "--short=8", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git sha of %s: %w", srcDir, err)
	}
	sha := strings.TrimSpace(string(out))
	return scheme(semver, sha, time.Now().UTC().Format("2006.01.02")), nil
}
