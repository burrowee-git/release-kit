// Package checksum writes SHA256SUMS files in the standard `shasum -a 256`
// format, using crypto/sha256 (no external tool) for reproducibility.
package checksum

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SumFile writes one "<hex>  <basename>\n" line per file to out, sorted by
// basename so the output is reproducible regardless of input order.
func SumFile(files []string, out string) error {
	type entry struct {
		name string
		line string
	}
	entries := make([]entry, 0, len(files))
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, fh)
		fh.Close()
		if err != nil {
			return err
		}
		name := filepath.Base(f)
		entries = append(entries, entry{name: name, line: fmt.Sprintf("%x  %s", h.Sum(nil), name)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = e.line
	}
	return os.WriteFile(out, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
