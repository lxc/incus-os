package main

import (
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/hex"
	"encoding/pem"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
)

//go:embed index.html.tmpl
var htmlTemplate string

type htmlPage struct {
	Title     string
	Updates   []htmlUpdate
	Keys      []htmlKey
	UpdatedAt string
}

type htmlUpdate struct {
	Version   string
	Published string
	Severity  string
	Channels  []htmlChannel
}

type htmlChannel struct {
	Name   string
	Latest bool
}

type htmlKey struct {
	Name        string
	Description string
	Fingerprint string
	PEM         string
	DER         string
}

// generateHTML writes a self-contained index.html listing the published images and the available keys.
func generateHTML(targetPath string, metaIndex apiupdate.Index) error {
	page := htmlPage{
		Title:     "Image server",
		UpdatedAt: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}

	// The index is sorted newest first, so the first image seen in a channel is its latest.
	latest := map[string]string{}

	for _, update := range metaIndex.Updates {
		for _, channel := range update.Channels {
			_, ok := latest[channel]
			if !ok {
				latest[channel] = update.Version
			}
		}
	}

	for _, update := range metaIndex.Updates {
		entry := htmlUpdate{
			Version:   update.Version,
			Published: update.PublishedAt.UTC().Format("2006-01-02 15:04 UTC"),
			Severity:  update.Severity.String(),
		}

		for _, file := range update.Files {
			if file.Component != apiupdate.UpdateFileComponentOS || file.Type != apiupdate.UpdateFileTypeImageRaw {
				continue
			}

			// Derive the OS name from the image file name.
			osName, _, found := strings.Cut(path.Base(file.Filename), "_")
			if found && page.Title == "Image server" {
				page.Title = osName + " image server"
			}
		}

		for _, channel := range sortChannels(update.Channels) {
			entry.Channels = append(entry.Channels, htmlChannel{Name: channel, Latest: latest[channel] == update.Version})
		}

		page.Updates = append(page.Updates, entry)
	}

	keys, err := indexKeys(filepath.Join(targetPath, "keys"))
	if err != nil {
		return err
	}

	page.Keys = keys

	tmpl, err := template.New("index").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	wr, err := os.Create(filepath.Join(targetPath, "index.html"))
	if err != nil {
		return err
	}

	defer func() { _ = wr.Close() }()

	return tmpl.Execute(wr, page)
}

// sortChannels returns a sorted copy of the channels, with the usual ones first.
func sortChannels(input []string) []string {
	channels := slices.Clone(input)
	preferred := []string{"stable", "testing", "daily"}

	slices.SortFunc(channels, func(a string, b string) int {
		aIndex := slices.Index(preferred, a)
		bIndex := slices.Index(preferred, b)

		if aIndex != bIndex {
			if aIndex == -1 {
				return 1
			}

			if bIndex == -1 {
				return -1
			}

			return aIndex - bIndex
		}

		return strings.Compare(a, b)
	})

	return channels
}

// indexKeys lists the Secure Boot certificates found in the keys directory, grouped by file name.
func indexKeys(keysPath string) ([]htmlKey, error) {
	entries, err := os.ReadDir(keysPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	keys := map[string]*htmlKey{}

	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if !slices.Contains([]string{".crt", ".pem", ".cer", ".der"}, ext) {
			continue
		}

		// Only list the Secure Boot certificates, those are the ones a user may need to enroll by hand.
		name := strings.TrimSuffix(entry.Name(), ext)
		if !strings.HasPrefix(name, "secureboot-PK-") && !strings.HasPrefix(name, "secureboot-KEK-") && !strings.HasPrefix(name, "secureboot-DB-") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(keysPath, entry.Name()))
		if err != nil {
			return nil, err
		}

		// Accept both PEM and DER regardless of the extension.
		isPEM := false

		block, _ := pem.Decode(content)
		if block != nil {
			content = block.Bytes
			isPEM = true
		}

		cert, err := x509.ParseCertificate(content)
		if err != nil {
			continue
		}

		key, ok := keys[name]
		if !ok {
			fingerprint := sha256.Sum256(cert.Raw)

			key = &htmlKey{
				Name:        name,
				Description: cert.Subject.CommonName,
				Fingerprint: formatFingerprint(hex.EncodeToString(fingerprint[:])),
			}

			keys[name] = key
		}

		if isPEM {
			key.PEM = entry.Name()
		} else {
			key.DER = entry.Name()
		}
	}

	result := []htmlKey{}

	for _, key := range keys {
		result = append(result, *key)
	}

	slices.SortFunc(result, func(a htmlKey, b htmlKey) int {
		return strings.Compare(a.Name, b.Name)
	})

	return result, nil
}

// formatFingerprint renders a hex digest as colon-separated uppercase bytes.
func formatFingerprint(digest string) string {
	parts := []string{}

	for i := 0; i+2 <= len(digest); i += 2 {
		parts = append(parts, strings.ToUpper(digest[i:i+2]))
	}

	return strings.Join(parts, ":")
}
