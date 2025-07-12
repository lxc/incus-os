// Package main is used for the image customizer.
package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/timpalpant/gzran"
	"gopkg.in/yaml.v3"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

const (
	imageTypeISO = "iso"
	imageTypeRaw = "raw"
)

var (
	images   map[string]apiImagesPost
	imagesMu sync.Mutex
)

type apiImagesPost struct {
	Type  string             `json:"type"  yaml:"type"`
	Seeds apiImagesPostSeeds `json:"seeds" yaml:"seeds"`
}

type apiImagesPostSeeds struct {
	Applications *apiseed.Applications `json:"applications" yaml:"applications"`
	Incus        *apiseed.Incus        `json:"incus"        yaml:"incus"`
	Install      *apiseed.Install      `json:"install"      yaml:"install"`
	Network      *apiseed.Network      `json:"network"      yaml:"network"`
	Provider     *apiseed.Provider     `json:"provider"     yaml:"provider"`
}

func main() {
	images = map[string]apiImagesPost{}

	err := do(context.TODO())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func do(_ context.Context) error {
	// Arguments.
	if len(os.Args) != 2 {
		return errors.New("missing image path")
	}

	// Check that image files exist.
	imagePath := os.Args[1]

	_, err := os.Stat(filepath.Join(imagePath, "image.iso.gz"))
	if err != nil {
		return fmt.Errorf("couldn't find 'image.iso.gz': %w", err)
	}

	_, err = os.Stat(filepath.Join(imagePath, "image.img.gz"))
	if err != nil {
		return fmt.Errorf("couldn't find 'image.img.gz': %w", err)
	}

	// Start REST server.
	listener, err := net.Listen("tcp", ":8080") //nolint:gosec
	if err != nil {
		return err
	}

	// Setup routing.
	router := http.NewServeMux()

	router.HandleFunc("/", apiRoot)
	router.HandleFunc("/1.0", apiRoot10)
	router.HandleFunc("/1.0/images", apiImages)
	router.HandleFunc("/1.0/images/{uuid}", apiImage)

	// Setup server.
	server := &http.Server{
		Handler: router,

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
	}

	return server.Serve(listener)
}

func apiRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	if r.URL.Path != "/" {
		_ = response.NotFound(nil).Render(w)

		return
	}

	_ = response.SyncResponse(true, []string{"/1.0"}).Render(w)
}

func apiRoot10(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	_ = response.SyncResponse(true, map[string]any{}).Render(w)
}

func apiImages(w http.ResponseWriter, r *http.Request) {
	// Confirm HTTP method.
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Parse the request.
	var req apiImagesPost

	err := yaml.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024)).Decode(&req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Validate input.
	if !slices.Contains([]string{imageTypeISO, imageTypeRaw}, req.Type) {
		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(errors.New("invalid image type")).Render(w)

		return
	}

	// Store the request.
	imagesMu.Lock()
	defer imagesMu.Unlock()

	imageUUID := uuid.New().String()

	images[imageUUID] = req

	// Return image details to the user.
	w.Header().Set("Content-Type", "application/json")

	err = response.SyncResponse(true, map[string]any{"image": "/1.0/images/" + imageUUID}).Render(w)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}
}

func apiImage(w http.ResponseWriter, r *http.Request) {
	// Confirm HTTP method.
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")

		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Image UUID.
	imageUUID := r.PathValue("uuid")

	imagesMu.Lock()

	req, ok := images[imageUUID]
	if ok {
		delete(images, imageUUID)
	}

	imagesMu.Unlock()

	if !ok {
		w.Header().Set("Content-Type", "application/json")

		_ = response.NotFound(nil).Render(w)

		return
	}

	// Determine source image name.
	var fileName string

	switch req.Type {
	case imageTypeISO:
		fileName = "image.iso.gz"
	case imageTypeRaw:
		fileName = "image.img.gz"
	}

	// Check if we have compression in-transit.
	compress := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

	// Open the image file.
	imageFile, err := os.Open(filepath.Join(os.Args[1], fileName)) //nolint:gosec
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = response.InternalError(err).Render(w)

		return
	}

	defer func() { _ = imageFile.Close() }()

	// Setup gzip seeking decompressor.
	rc, err := gzran.NewReader(imageFile)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = response.InternalError(err).Render(w)

		return
	}

	// Track down image file.
	fileTarget, err := os.Readlink(filepath.Join(os.Args[1], fileName))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = response.InternalError(err).Render(w)

		return
	}

	fileName = filepath.Base(fileTarget)

	// Serve the image.
	if compress {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/octet-stream")

		fileName = strings.TrimSuffix(fileName, ".gz")
	} else {
		w.Header().Set("Content-Type", "application/gzip")
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	// Setup compressor.
	writer := gzip.NewWriter(w)
	defer writer.Close()

	// Write leading part.
	remainder := int64(2148532224)
	for {
		chunk := int64(4 * 1024 * 1024)
		if remainder < chunk {
			chunk = remainder
		}

		if chunk == 0 {
			break
		}

		n, err := io.CopyN(writer, rc, chunk)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return
		}

		remainder -= n
	}

	// Write seed file.
	seedSize, err := writeSeed(writer, req.Seeds)
	if err != nil {
		return
	}

	// Write trailing part.
	_, err = rc.Seek(int64(seedSize), 1)
	if err != nil {
		return
	}

	for {
		_, err = io.CopyN(writer, rc, 4*1024*1024)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return
		}
	}
}

func writeSeed(writer io.Writer, seeds apiImagesPostSeeds) (int, error) {
	archiveContents := [][]string{}

	// Create applications yaml contents.
	if seeds.Applications != nil {
		yamlContents, err := yaml.Marshal(seeds.Applications)
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"applications.yaml", string(yamlContents)})
	}

	// Create incus yaml contents.
	if seeds.Incus != nil {
		yamlContents, err := yaml.Marshal(seeds.Incus)
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"incus.yaml", string(yamlContents)})
	}

	// Create install yaml contents.
	if seeds.Install != nil {
		yamlContents, err := yaml.Marshal(seeds.Install)
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"install.yaml", string(yamlContents)})
	}

	// Create network yaml contents.
	if seeds.Network != nil {
		yamlContents, err := yaml.Marshal(seeds.Network)
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"network.yaml", string(yamlContents)})
	}

	// Create provider yaml contents.
	if seeds.Provider != nil {
		yamlContents, err := yaml.Marshal(seeds.Provider)
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"provider.yaml", string(yamlContents)})
	}

	// Put a size counter in place.
	wc := &writeCounter{}

	// Create the tar archive.
	tw := tar.NewWriter(io.MultiWriter(wc, writer))

	for _, file := range archiveContents {
		hdr := &tar.Header{
			Name: file[0],
			Mode: 0o600,
			Size: int64(len(file[1])),
		}

		err := tw.WriteHeader(hdr)
		if err != nil {
			return -1, err
		}

		_, err = tw.Write([]byte(file[1]))
		if err != nil {
			return -1, err
		}
	}

	err := tw.Close()
	if err != nil {
		return -1, err
	}

	return wc.size, nil
}

type writeCounter struct {
	size int
}

func (wc *writeCounter) Write(buf []byte) (int, error) {
	size := len(buf)
	wc.size += size

	return size, nil
}
