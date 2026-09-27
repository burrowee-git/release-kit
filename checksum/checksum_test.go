package checksum

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSums(t *testing.T) {
	type input struct{ name, content string }
	cases := []struct {
		name  string
		files []input
		want  string
	}{
		{
			name:  "input_order_ignored",
			files: []input{{"b.txt", "world\n"}, {"a.txt", "hello\n"}},
			want: "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03  a.txt\n" +
				"e258d248fda94c63753607f7c4494ee0fcbe92f1a76bfdac795c9d84101eb317  b.txt\n",
		},
		{
			name:  "basename_not_hash",
			files: []input{{"a.txt", "2"}, {"z.txt", "1"}},
			want: "d4735e3a265e16eee03f59718b9b5d03019c07d8b6c51f90da3a666eec13ab35  a.txt\n" +
				"6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b  z.txt\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var paths []string
			for _, f := range tc.files {
				p := filepath.Join(dir, f.name)
				if err := os.WriteFile(p, []byte(f.content), 0o644); err != nil {
					t.Fatal(err)
				}
				paths = append(paths, p)
			}
			out := filepath.Join(dir, "SHA256SUMS.txt")
			if err := WriteSums(paths, out); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("mismatch:\n got=%q\nwant=%q", got, tc.want)
			}
		})
	}
}

func TestWriteSumsRejectsDuplicateBasenames(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(dir, "tool")
	b := filepath.Join(sub, "tool")
	if err := os.WriteFile(a, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "SHA256SUMS.txt")
	err := WriteSums([]string{a, b}, out)
	if err == nil {
		t.Fatal("expected error for duplicate basename, got nil")
	}
	if !strings.Contains(err.Error(), "tool") {
		t.Errorf("error %q does not mention the colliding basename", err)
	}
}

func TestWriteSumsRejectsUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.bin")
	err := WriteSums([]string{missing}, filepath.Join(dir, "SHA256SUMS.txt"))
	if err == nil || !strings.Contains(err.Error(), "checksum: hash") {
		t.Errorf("WriteSums error = %v, want a checksum: hash error", err)
	}
}
