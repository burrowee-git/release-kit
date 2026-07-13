package checksum

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSumFileSortedAndCorrect(t *testing.T) {
	dir := t.TempDir()
	// "hello\n" → 5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
	// "world\n" → e258d248fda94c63753607f7c4494ee0fcbe92f1a76bfdac795c9d84101eb317
	b := filepath.Join(dir, "b.txt")
	a := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(b, []byte("world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "SHA256SUMS.txt")
	// pass b before a to prove sorting by basename
	if err := SumFile([]string{b, a}, out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03  a.txt\n" +
		"e258d248fda94c63753607f7c4494ee0fcbe92f1a76bfdac795c9d84101eb317  b.txt\n"
	if string(got) != want {
		t.Errorf("mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestSumFileSortsByBasenameNotHash(t *testing.T) {
	dir := t.TempDir()
	// "2" (no newline) → d4735e3a265e16eee03f59718b9b5d03019c07d8b6c51f90da3a666eec13ab35
	// "1" (no newline) → 6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b
	// Basename order: a.txt, z.txt. Hash order: z.txt (6b…) before a.txt (d4…).
	a := filepath.Join(dir, "a.txt")
	z := filepath.Join(dir, "z.txt")
	if err := os.WriteFile(a, []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(z, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "SHA256SUMS.txt")
	if err := SumFile([]string{a, z}, out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "d4735e3a265e16eee03f59718b9b5d03019c07d8b6c51f90da3a666eec13ab35  a.txt\n" +
		"6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b  z.txt\n"
	if string(got) != want {
		t.Errorf("mismatch (basename sort expected):\n got=%q\nwant=%q", got, want)
	}
}
