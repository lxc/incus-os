package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

var seedExts = []string{"json", "yaml", "yml"}

var seedTargets = map[string]any{
	"install":           &apiseed.Install{},
	"applications":      &apiseed.Applications{},
	"incus":             &apiseed.Incus{},
	"kernel":            &apiseed.Kernel{},
	"network":           &apiseed.Network{},
	"migration-manager": &apiseed.MigrationManager{},
	"operations-center": &apiseed.OperationsCenter{},
	"provider":          &apiseed.Provider{},
	"security":          &apiseed.Security{},
	"services":          &apiseed.Services{},
	"storage":           &apiseed.Storage{},
	"update":            &apiseed.Update{},
}

// formatSection properly indents a text section.
func formatSection(header string, content string) string {
	var out strings.Builder

	// Add section header
	if header != "" {
		_, _ = out.WriteString(header + ":\n")
	}

	// Indent the content
	for line := range strings.SplitSeq(content, "\n") {
		if line != "" {
			_, _ = out.WriteString("  ")
		}

		_, _ = out.WriteString(line + "\n")
	}

	if header != "" {
		// Section separator (when rendering a full section
		_, _ = out.WriteString("\n")

		return out.String()
	}

	// Remove last newline when rendering partial section
	return strings.TrimSuffix(out.String(), "\n")
}

// loadSeed load the user defined seed tar.
func loadSeed(filename string) ([]byte, error) {
	var (
		b        []byte
		fi       os.FileInfo
		err      error
		hdr      *tar.Header
		target   any
		found    bool
		name     string
		ext      string
		seedName string
		content  string
		buf      *bytes.Buffer
		written  int64
		tr       = tar.NewReader(bytes.NewReader(b))
		names    = []string{}
	)

	fi, err = os.Stat(filename)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		err = fmt.Errorf("%s is not tar file", filename)

		return nil, err
	}

	b, err = os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	l := len(b)
	if l/512 < 3 {
		err = fmt.Errorf("%q not valid tar file", filename)

		return nil, err
	}

	for {
		hdr, err = tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			err = fmt.Errorf("%q not valid tar format: %w", filename, err)

			return nil, err
		}

		name = filepath.Clean(hdr.Name)
		if name != filepath.Base(name) || hdr.FileInfo().IsDir() {
			err = fmt.Errorf("%q in %q should not contain any directory", name, filename)

			return nil, err
		}

		ext = filepath.Ext(name)
		if ext == "" {
			err = fmt.Errorf("%q has no extension", name)

			return nil, err
		}

		seedName = strings.TrimSuffix(name, ext)

		ext = ext[1:]
		if !slices.Contains(seedExts, ext) {
			err = fmt.Errorf("%q not in support list %q", ext, seedExts)

			return nil, err
		}

		target, found = seedTargets[seedName]
		if !found {
			err = fmt.Errorf("%q not in support", seedName)

			return nil, err
		}

		if slices.Contains(names, seedName) {
			err = fmt.Errorf("%q is duplicated in tar %q", seedName, filename)

			return nil, err
		}

		names = append(names, seedName)
		buf = &bytes.Buffer{}

		written, err = io.CopyN(buf, tr, hdr.Size)
		if err != nil {
			err = fmt.Errorf("invalid tar format %q, failed to copy: %w", filename, err)

			return nil, err
		}

		if written != hdr.Size {
			err = fmt.Errorf("invalid tar format %q, size mismatch %d != %d", filename, written, hdr.Size)

			return nil, err
		}

		content = buf.String()

		err = validateContent(buf, ext, target)
		if err != nil {
			err = fmt.Errorf("failed to validate %q in %q with content %q: %w", name, filename, content, err)

			return nil, err
		}
	}

	return b, nil
}

func validateContent(r io.Reader, ext string, target any) error {
	var (
		decoder *json.Decoder
		loader  *yaml.Loader
		err     error
		decode  func(any) error
	)

	if ext == "json" {
		decoder = json.NewDecoder(r)
		decoder.DisallowUnknownFields()
		decode = decoder.Decode
	} else {
		loader, err = yaml.NewLoader(r, yaml.WithKnownFields())
		if err != nil {
			return err
		}

		decode = loader.Load
	}

	return decode(target)
}
