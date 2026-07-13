// Package checksum writes SHA256SUMS files in the standard `shasum -a 256`
// format, using crypto/sha256 (no external tool) for reproducibility.
package checksum

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// WriteSums writes one "<hex>  <basename>\n" line per file to out, sorted by
// basename so the output is reproducible regardless of input order. Input
// files must have distinct basenames; two files sharing a basename would
// produce an ambiguous SHA256SUMS and return an error instead.
func WriteSums(files []string, out string) error {
	type entry struct {
		name string
		line string
	}
	entries := make([]entry, 0, len(files))
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		name := filepath.Base(f)
		if seen[name] {
			return fmt.Errorf("checksum: duplicate basename %q (ambiguous SHA256SUMS)", name)
		}
		seen[name] = true

		sum, err := hashFile(f)
		if err != nil {
			return fmt.Errorf("checksum: hash %s: %w", f, err)
		}
		entries = append(entries, entry{name: name, line: fmt.Sprintf("%x  %s", sum, name)})
	}
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.name, b.name) })
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = e.line
	}
	return os.WriteFile(out, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func hashFile(f string) ([]byte, error) {
	fh, err := os.Open(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fh.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, fh); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
