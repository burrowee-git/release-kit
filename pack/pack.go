package pack

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Content struct {
	Src  string
	Name string
}

type Spec struct {
	Contents []Content
	Out      string
}

func Zip(spec Spec) (err error) {
	zf, ferr := os.Create(spec.Out)
	if ferr != nil {
		return fmt.Errorf("pack: create %s: %w", spec.Out, ferr)
	}
	defer func() {
		if cerr := zf.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("pack: close %s: %w", spec.Out, cerr)
		}
	}()

	zw := zip.NewWriter(zf)
	seen := make(map[string]bool, len(spec.Contents))
	for _, c := range spec.Contents {
		name := c.Name
		if name == "" {
			name = filepath.Base(c.Src)
		}
		if sanitizeErr := validateName(name); sanitizeErr != nil {
			return fmt.Errorf("pack: %s: %w", c.Src, sanitizeErr)
		}
		if seen[name] {
			return fmt.Errorf("pack: %s: duplicate in-archive name %q", c.Src, name)
		}
		seen[name] = true

		if entryErr := writeEntry(zw, c, name); entryErr != nil {
			return entryErr
		}
	}
	if closeErr := zw.Close(); closeErr != nil {
		return fmt.Errorf("pack: %s: %w", spec.Out, closeErr)
	}
	return nil
}

func writeEntry(zw *zip.Writer, c Content, name string) error {
	info, statErr := os.Stat(c.Src)
	if statErr != nil {
		return fmt.Errorf("pack: %s: %w", c.Src, statErr)
	}
	hdr, hdrErr := zip.FileInfoHeader(info)
	if hdrErr != nil {
		return fmt.Errorf("pack: %s: %w", c.Src, hdrErr)
	}
	hdr.Name = name
	hdr.Method = zip.Deflate
	w, createErr := zw.CreateHeader(hdr)
	if createErr != nil {
		return fmt.Errorf("pack: %s: %w", c.Src, createErr)
	}
	src, openErr := os.Open(c.Src)
	if openErr != nil {
		return fmt.Errorf("pack: %s: %w", c.Src, openErr)
	}
	_, copyErr := io.Copy(w, src)
	_ = src.Close()
	if copyErr != nil {
		return fmt.Errorf("pack: %s: %w", c.Src, copyErr)
	}
	return nil
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("empty in-archive name")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return fmt.Errorf("absolute in-archive name %q", name)
	}
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == ".." {
			return fmt.Errorf("in-archive name %q contains \"..\"", name)
		}
	}
	return nil
}
