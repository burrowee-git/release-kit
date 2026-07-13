package minisign

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSignVerifyRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("minisign"); err != nil {
		t.Skip("minisign not installed")
	}
	dir := t.TempDir()
	sec := filepath.Join(dir, "key.sec")
	pub := filepath.Join(dir, "key.pub")
	// -W → password-less key (no interactive prompt)
	if out, err := exec.Command("minisign", "-G", "-W", "-p", pub, "-s", sec).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v\n%s", err, out)
	}
	sums := filepath.Join(dir, "SHA256SUMS.txt")
	if err := os.WriteFile(sums, []byte("deadbeef  x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Sign(sums, sec); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := os.Stat(sums + ".minisig"); err != nil {
		t.Fatalf("no .minisig written: %v", err)
	}
	if err := Verify(sums, pub); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	// tamper → verify must fail
	if err := os.WriteFile(sums, []byte("tampered  x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Verify(sums, pub); err == nil {
		t.Fatal("Verify passed a tampered file")
	}
}
