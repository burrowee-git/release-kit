package version

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBump(t *testing.T) {
	cases := []struct {
		name string
		cur  string
		kind BumpKind
		want string
	}{
		{name: "patch", cur: "0.1.9", kind: BumpPatch, want: "0.1.10"},
		{name: "minor", cur: "0.1.9", kind: BumpMinor, want: "0.2.0"},
		{name: "major", cur: "1.4.2", kind: BumpMajor, want: "2.0.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Bump(tc.cur, tc.kind)
			if err != nil || got != tc.want {
				t.Errorf("Bump(%q,%v)=%q,%v want %q", tc.cur, tc.kind, got, err, tc.want)
			}
		})
	}
}

func TestDateVersionScheme(t *testing.T) {
	if got := DateVersionScheme("0.1.0", "abcd1234", "2026.07.12"); got != "v0.1.0.2026.07.12.abcd1234" {
		t.Errorf("got %q", got)
	}
}

func TestStamp(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "c")
	semFile := filepath.Join(dir, "ver")
	if err := os.WriteFile(semFile, []byte("0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := time.Now().UTC().Format("2006.01.02")
	got, err := Stamp(context.Background(), semFile, dir, DateVersionScheme)
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC().Format("2006.01.02")
	prefix := "v0.1.0." + before + "."
	if !strings.HasPrefix(got, prefix) {
		prefix = "v0.1.0." + after + "."
	}
	if !strings.HasPrefix(got, prefix) {
		t.Errorf("stamp %q missing v0.1.0.<UTC date>. prefix (date %s or %s)", got, before, after)
	}
	if len(strings.TrimPrefix(got, prefix)) != 8 {
		t.Errorf("stamp %q sha8 suffix wrong length", got)
	}
}

func TestStampGitErrorIncludesStderr(t *testing.T) {
	dir := t.TempDir()
	semFile := filepath.Join(dir, "ver")
	if err := os.WriteFile(semFile, []byte("0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Stamp(context.Background(), semFile, dir, DateVersionScheme)
	if err == nil {
		t.Fatal("expected error for a non-git srcDir")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error %q missing git stderr diagnostic", err.Error())
	}
}

func TestBumpRejects(t *testing.T) {
	cases := []struct {
		name string
		cur  string
		kind BumpKind
	}{
		{name: "not_semver", cur: "notsemver", kind: BumpPatch},
		{name: "major_overflow", cur: "99999999999999999999.0.0", kind: BumpPatch},
		{name: "minor_overflow", cur: "0.99999999999999999999.0", kind: BumpPatch},
		{name: "patch_overflow", cur: "0.0.99999999999999999999", kind: BumpPatch},
		{name: "unknown_kind", cur: "0.1.9", kind: BumpKind(99)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := Bump(tc.cur, tc.kind); err == nil {
				t.Errorf("Bump(%q, %v) = %q, nil; want an error", tc.cur, tc.kind, got)
			}
		})
	}
}

func TestStampRejects(t *testing.T) {
	dir := t.TempDir()
	notSemver := filepath.Join(dir, "ver")
	if err := os.WriteFile(notSemver, []byte("1.2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		semverFile string
		says       string
	}{
		{name: "missing_file", semverFile: filepath.Join(dir, "absent"), says: "version: read"},
		{name: "not_semver", semverFile: notSemver, says: "not MAJOR.MINOR.PATCH"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Stamp(context.Background(), tc.semverFile, dir, DateVersionScheme)
			if err == nil || !strings.Contains(err.Error(), tc.says) {
				t.Errorf("Stamp error = %v, want one containing %q", err, tc.says)
			}
		})
	}
}
