// Package main is used for the image customizer.
package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
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

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

//go:embed html
var staticFiles embed.FS

const (
	imageArchitectureX86_64  = "x86_64"
	imageArchitectureAARCH64 = "aarch64"

	imageTypeISO = "iso"
	imageTypeRaw = "raw"
)

var (
	images   map[string]apiImagesPost
	imagesMu sync.Mutex
)

type apiImagesPost struct {
	Architecture string             `json:"architecture" yaml:"architecture"`
	Type         string             `json:"type"         yaml:"type"`
	Seeds        apiImagesPostSeeds `json:"seeds"        yaml:"seeds"`
}

type apiImagesPostSeeds struct {
	Applications     *apiseed.Applications     `json:"applications"      yaml:"applications"`
	Incus            *apiseed.Incus            `json:"incus"             yaml:"incus"`
	Install          *apiseed.Install          `json:"install"           yaml:"install"`
	MigrationManager *apiseed.MigrationManager `json:"migration-manager" yaml:"migration-manager"` //nolint:tagliatelle
	OperationsCenter *apiseed.OperationsCenter `json:"operations-center" yaml:"operations-center"` //nolint:tagliatelle
	Network          *apiseed.Network          `json:"network"           yaml:"network"`
	Provider         *apiseed.Provider         `json:"provider"          yaml:"provider"`
}

func main() {
	images = map[string]apiImagesPost{}

	err := do(context.TODO())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		os.Exit(1)
	}
}

func do(ctx context.Context) error {
	// Arguments.
	if len(os.Args) != 2 {
		return errors.New("missing image path")
	}

	// Start REST server.
	lc := &net.ListenConfig{}

	listener, err := lc.Listen(ctx, "tcp", ":8080")
	if err != nil {
		return err
	}

	// Server the embedded pages.
	fsUI, err := fs.Sub(fs.FS(staticFiles), "html")
	if err != nil {
		return err
	}

	// Setup routing.
	router := http.NewServeMux()

	router.HandleFunc("/", apiRoot)
	router.HandleFunc("/1.0", apiRoot10)
	router.HandleFunc("/1.0/images", apiImages)
	router.HandleFunc("/1.0/images/{uuid}", apiImage)
	router.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(fsUI))))

	// Setup server.
	server := &http.Server{
		Handler: router,

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
	}

	return server.Serve(listener)
}

func clientAddress(r *http.Request) string {
	if r.Header.Get("x-forwarded-for") != "" {
		return r.Header.Get("x-forwarded-for")
	}

	return r.RemoteAddr
}

func apiRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	if strings.Contains(r.Header.Get("User-Agent"), "Gecko") {
		// Redirect to UI.
		http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)

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
	// Set CORS headers.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Confirm HTTP method.
	if r.Method == http.MethodOptions {
		return
	} else if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Parse the request.
	var req apiImagesPost

	err := yaml.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024)).Decode(&req)
	if err != nil {
		slog.Warn("image request: bad JSON", "client", clientAddress(r), "err", err)

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

	if !slices.Contains([]string{imageArchitectureX86_64, imageArchitectureAARCH64}, req.Architecture) {
		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(errors.New("invalid image architecture")).Render(w)

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

	slog.Info("image request: created", "client", clientAddress(r), "type", req.Type, "architecture", req.Architecture)
}

func apiImage(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Confirm HTTP method.
	if r.Method == http.MethodOptions {
		return
	} else if r.Method != http.MethodGet {
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

	// Determine source image type.
	var fileType string

	switch req.Type {
	case imageTypeISO:
		fileType = "image-iso"
	case imageTypeRaw:
		fileType = "image-raw"
	default:
		_ = response.BadRequest(nil).Render(w)

		return
	}

	// Find latest image.
	var metaIndex apiupdate.Index

	metaFile, err := os.Open(filepath.Join(os.Args[1], "index.json"))
	if err != nil {
		slog.Warn("image retrieve: bad index", "client", clientAddress(r), "err", err)

		_ = response.InternalError(err).Render(w)

		return
	}

	defer func() { _ = metaFile.Close() }()

	err = json.NewDecoder(metaFile).Decode(&metaIndex)
	if err != nil {
		slog.Warn("image retrieve: bad index", "client", clientAddress(r), "err", err)

		_ = response.InternalError(err).Render(w)

		return
	}

	var imageFilePath string

	for _, update := range metaIndex.Updates {
		if !slices.Contains(update.Channels, "stable") {
			continue
		}

		for _, fileEntry := range update.Files {
			if string(fileEntry.Architecture) == req.Architecture && string(fileEntry.Type) == fileType {
				imageFilePath = filepath.Join(os.Args[1], update.Version, fileEntry.Filename)

				break
			}
		}

		if imageFilePath != "" {
			break
		}
	}

	if imageFilePath == "" {
		slog.Warn("image retrieve: image not found", "client", clientAddress(r))

		_ = response.InternalError(errors.New("couldn't find matching image")).Render(w)

		return
	}

	// Check if we have compression in-transit.
	compress := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

	// Open the image file.
	imageFile, err := os.Open(imageFilePath) //nolint:gosec
	if err != nil {
		slog.Warn("image retrieve: bad image", "client", clientAddress(r), "err", err)

		w.Header().Set("Content-Type", "application/json")
		_ = response.InternalError(err).Render(w)

		return
	}

	defer func() { _ = imageFile.Close() }()

	// Setup gzip seeking decompressor.
	rc, err := gzran.NewReader(imageFile)
	if err != nil {
		slog.Warn("image retrieve: bad image", "client", clientAddress(r), "err", err)

		w.Header().Set("Content-Type", "application/json")
		_ = response.InternalError(err).Render(w)

		return
	}

	// Track down image file.
	fileName := filepath.Base(imageFilePath)

	// Serve the image.
	if compress {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/octet-stream")

		fileName = strings.TrimSuffix(fileName, ".gz")
	} else {
		w.Header().Set("Content-Type", "application/gzip")
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	w.WriteHeader(http.StatusOK)

	// Setup compressor.
	writer := gzip.NewWriter(w)
	defer writer.Close()

	// Write leading part.
	remainder := int64(2148532224)

	for {
		chunk := min(remainder, int64(4*1024*1024))

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

	slog.Info("image retrieve: retrieved", "client", clientAddress(r), "type", req.Type, "architecture", req.Architecture)
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

	// Create operations-center yaml contents.
	if seeds.OperationsCenter != nil {
		yamlContents, err := yaml.Marshal(seeds.OperationsCenter)
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"operations-center.yaml", string(yamlContents)})
	}

	// Create migration-manager yaml contents.
	if seeds.MigrationManager != nil {
		yamlContents, err := yaml.Marshal(seeds.MigrationManager)
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"migration-manager.yaml", string(yamlContents)})
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
