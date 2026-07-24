// Package main is used for the image customizer.
package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"embed"
	"encoding/base64"
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
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v7/shared/subprocess"
	"github.com/pires/go-proxyproto"
	"github.com/timpalpant/gzran"
	"go.yaml.in/yaml/v4"

	apicustomizer "github.com/lxc/incus-os/incus-osd/api/customizer"
	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

//go:embed html
var staticFiles embed.FS

const (
	imageArchitectureX86_64  = "x86_64"
	imageArchitectureAARCH64 = "aarch64"

	imageTypeISO = "iso"
	imageTypeRaw = "raw"

	osFile     = "os"
	updateFile = "update"
	rescueFile = "rescue"
)

type imageOptions struct {
	Type string
	Data []byte
}

var files *util.TTLMap[string, imageOptions]

func main() {
	files = util.NewTTLMap[string, imageOptions](context.TODO(), time.Second)

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

	// Setup the listener.
	lc := &net.ListenConfig{}

	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")

	listenAddress := os.Getenv("LISTEN_ADDRESS")
	if listenAddress == "" {
		if tlsCert != "" && tlsKey != "" {
			listenAddress = ":8443"
		} else {
			listenAddress = ":8080"
		}
	}

	listener, err := lc.Listen(ctx, "tcp", listenAddress)
	if err != nil {
		return err
	}

	// Support proxy protocol (optional, plain connections remain allowed).
	proxyListener := &proxyproto.Listener{
		Listener: listener,
		ConnPolicy: func(_ proxyproto.ConnPolicyOptions) (proxyproto.Policy, error) {
			return proxyproto.USE, nil
		},
	}
	defer proxyListener.Close()

	listener = proxyListener

	// Server the embedded pages.
	fsUI, err := fs.Sub(fs.FS(staticFiles), "html")
	if err != nil {
		return err
	}

	// Setup routing.
	router := http.NewServeMux()

	router.HandleFunc("/", apiRoot)
	router.HandleFunc("/1.0", apiRoot10)
	router.HandleFunc("/1.0/certificate", apiCertificate)
	router.HandleFunc("/1.0/images", apiImages)
	router.HandleFunc("/1.0/updates", apiUpdates)
	router.HandleFunc("/1.0/files/{uuid}", apiFiles)
	router.HandleFunc("/1.0/oidc", apiOIDC)
	router.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(fsUI))))

	// Setup server.
	server := &http.Server{
		Handler: router,

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
	}

	if tlsCert != "" && tlsKey != "" {
		return server.ServeTLS(proxyListener, tlsCert, tlsKey)
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
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

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
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	_ = response.SyncResponse(true, map[string]any{}).Render(w)
}

func apiCertificate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Generate the certificate.
	cert, key, pfx, err := certificateGenerate()
	if err != nil {
		slog.Warn("certificate request: failed generation", "client", clientAddress(r), "err", err)

		w.Header().Set("Content-Type", "application/json")
		_ = response.InternalError(err).Render(w)

		return
	}

	slog.Info("certificate generated", "client", clientAddress(r))

	resp := apicustomizer.CertificateGet{
		Certificate: string(cert),
		Key:         string(key),
		PFX:         base64.StdEncoding.EncodeToString(pfx),
	}

	_ = response.SyncResponse(true, resp).Render(w)
}

func apiOIDC(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Generate the OIDC credentials.
	userName := r.FormValue("username")

	issuer, clientID, err := oidcGenerate(r.Context(), userName)
	if err != nil {
		slog.Warn("oidc request: failed generation", "client", clientAddress(r), "err", err)

		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(err).Render(w)

		return
	}

	slog.Info("oidc generated", "client", clientAddress(r), "username", userName)

	resp := apicustomizer.OIDCGet{
		Issuer:   issuer,
		ClientID: clientID,
	}

	_ = response.SyncResponse(true, resp).Render(w)
}

func apiImages(w http.ResponseWriter, r *http.Request) {
	recordFileRequest(w, r, osFile)
}

func apiUpdates(w http.ResponseWriter, r *http.Request) {
	recordFileRequest(w, r, updateFile)
}

func recordFileRequest(w http.ResponseWriter, r *http.Request, fileType string) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

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

	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024*1024))
	if err != nil {
		slog.Warn("image request: bad loader", "client", clientAddress(r), "err", err)

		w.Header().Set("Content-Type", "application/json")
		_ = response.InternalError(err).Render(w)

		return
	}

	// Store the request.
	resp := map[string]string{}

	switch fileType {
	case osFile:
		var req apicustomizer.ImagesPost

		err := json.Unmarshal(b, &req)
		if err != nil {
			slog.Warn("image request: request data", "client", clientAddress(r), "err", err)

			w.Header().Set("Content-Type", "application/json")
			_ = response.InternalError(err).Render(w)

			return
		}

		imageUUID := uuid.New().String()
		files.Set(imageUUID, imageOptions{Data: b, Type: osFile}, time.Minute*10, nil)

		resp["image"] = "/1.0/files/" + imageUUID

		if req.Offline {
			resourcesUUID := uuid.New().String()
			files.Set(resourcesUUID, imageOptions{Data: b, Type: rescueFile}, time.Minute*10, nil)
			resp["resources"] = "/1.0/files/" + resourcesUUID
		}

	case updateFile:
		updateUUID := uuid.New().String()
		files.Set(updateUUID, imageOptions{Data: b, Type: updateFile}, time.Minute*10, nil)
		resp["update"] = "/1.0/files/" + updateUUID
	default:
		slog.Warn("image request: bad image type", "client", clientAddress(r), "type", fileType)

		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(fmt.Errorf("unknown image type %q", fileType)).Render(w)

		return
	}

	// Return image details to the user.
	w.Header().Set("Content-Type", "application/json")

	err = response.SyncResponse(true, resp).Render(w)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	slog.Info("image request: created", "client", clientAddress(r))
}

func apiFiles(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

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

	opts, ok := files.Get(imageUUID)
	if !ok {
		w.Header().Set("Content-Type", "application/json")

		_ = response.NotFound(nil).Render(w)

		return
	}

	files.Delete(imageUUID)

	switch opts.Type {
	case rescueFile:
		sendRescueImage(w, r, imageUUID, opts.Data)
	case updateFile:
		sendUpdateTarball(w, r, opts.Data)
	case osFile:
		sendOSImage(w, r, opts.Data)
	default:
		w.Header().Set("Content-Type", "application/json")

		_ = response.NotFound(nil).Render(w)

		return
	}
}

func sendOSImage(w http.ResponseWriter, r *http.Request, b []byte) {
	var req apicustomizer.ImagesPost

	err := json.Unmarshal(b, &req)
	if err != nil {
		slog.Warn("image parse: bad request data", "client", clientAddress(r), "err", err)
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Determine source image type.
	var fileType apiupdate.UpdateFileType

	switch req.Type {
	case imageTypeISO:
		fileType = apiupdate.UpdateFileTypeImageISO
	case imageTypeRaw:
		fileType = apiupdate.UpdateFileTypeImageRaw
	default:
		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(fmt.Errorf("unknown file type %q", req.Type)).Render(w)

		return
	}

	if !slices.Contains([]apiupdate.UpdateFileArchitecture{imageArchitectureX86_64, imageArchitectureAARCH64}, req.Architecture) {
		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(errors.New("invalid image architecture")).Render(w)

		return
	}

	// Set default values.
	if req.Channel == "" {
		req.Channel = "stable"
	}

	metaIndex, err := parseIndex()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(err).Render(w)

		return
	}

	version, assets, err := filterAssets(*metaIndex, apicustomizer.UpdateFilter{
		Channel:       req.Channel,
		Version:       req.Version,
		Components:    []apiupdate.UpdateFileComponent{},
		Types:         []apiupdate.UpdateFileType{fileType},
		Architectures: []apiupdate.UpdateFileArchitecture{req.Architecture},
	})
	if err != nil || len(assets) != 1 {
		log := slog.Default()
		if err != nil {
			log = log.With("err", err)
		}

		log.Warn("image retrieve: failed asset lookup", "client", clientAddress(r))

		_ = response.InternalError(errors.New("couldn't find matching image")).Render(w)

		return
	}

	imageFilePath := filepath.Join(os.Args[1], version, assets[0])

	// Open the image file.
	imageFile, err := os.Open(imageFilePath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		slog.Warn("image retrieve: bad image", "client", clientAddress(r), "err", err)
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
	setFileHeaders(w, r, fileName)

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

func sendRescueImage(w http.ResponseWriter, r *http.Request, imageUUID string, b []byte) {
	var req apicustomizer.ImagesPost

	err := json.Unmarshal(b, &req)
	if err != nil {
		slog.Warn("image parse: bad request data", "client", clientAddress(r), "err", err)
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Determine source image type.
	switch req.Type {
	case imageTypeISO, imageTypeRaw:
	default:
		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(fmt.Errorf("unknown image type: %q", req.Type)).Render(w)

		return
	}

	if !slices.Contains([]apiupdate.UpdateFileArchitecture{imageArchitectureX86_64, imageArchitectureAARCH64}, req.Architecture) {
		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(errors.New("invalid image architecture")).Render(w)

		return
	}

	// Set default values.
	if req.Channel == "" {
		req.Channel = "stable"
	}

	metaIndex, err := parseIndex()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(err).Render(w)

		return
	}

	var applications []apiupdate.UpdateFileComponent

	if req.Seeds.Applications != nil {
		for _, seed := range req.Seeds.Applications.Applications {
			applications = append(applications, apiupdate.UpdateFileComponent(seed.Name))
		}
	}

	if len(applications) == 0 {
		w.Header().Set("Content-Type", "application/json")
		slog.Warn("image retrieve: no application seed data found", "client", clientAddress(r))

		_ = response.InternalError(errors.New("couldn't find matching update")).Render(w)
	}

	version, assets, err := filterAssets(*metaIndex, apicustomizer.UpdateFilter{
		Channel:       req.Channel,
		Version:       req.Version,
		Architectures: []apiupdate.UpdateFileArchitecture{req.Architecture},
		Types:         []apiupdate.UpdateFileType{apiupdate.UpdateFileTypeApplication},
		Components:    applications,
	})
	if err != nil || len(assets) != len(applications) {
		log := slog.Default()
		if err != nil {
			log = slog.With("err", err)
		}

		w.Header().Set("Content-Type", "application/json")
		log.Warn("image retrieve: applications error", "client", clientAddress(r))

		_ = response.InternalError(errors.New("couldn't find matching update")).Render(w)

		return
	}

	assets = append(assets, "update.json", "update.sjson")

	tempFile := filepath.Join("/tmp", "rescue-"+imageUUID+"."+req.Type)
	defer os.Remove(tempFile)

	err = buildImage(imageUUID, req.Type, tempFile, version, assets)
	if err != nil {
		slog.Warn("image build: failed to build image", "client", clientAddress(r), "err", err)
		_ = response.BadRequest(err).Render(w)

		return
	}

	fileName := req.Channel + "-" + version + "." + req.Type + ".gz"
	setFileHeaders(w, r, fileName)

	// Setup compressor.
	writer := gzip.NewWriter(w)
	defer writer.Close()

	rc, err := os.Open(tempFile)
	if err != nil {
		slog.Warn("image write: failed to open image file", "client", clientAddress(r), "err", err)
		_ = response.BadRequest(err).Render(w)

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

func sendUpdateTarball(w http.ResponseWriter, r *http.Request, b []byte) {
	var req apicustomizer.UpdatesPost

	err := json.Unmarshal(b, &req)
	if err != nil {
		slog.Warn("image parse: bad request data", "client", clientAddress(r), "err", err)
		_ = response.BadRequest(err).Render(w)

		return
	}
	// Set default values.
	if req.Channel == "" {
		req.Channel = "stable"
	}

	metaIndex, err := parseIndex()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = response.BadRequest(err).Render(w)

		return
	}

	version, assets, err := filterAssets(*metaIndex, req.UpdateFilter)
	if err != nil || len(assets) == 0 {
		log := slog.Default()
		if err != nil {
			log = slog.With("err", err)
		}

		w.Header().Set("Content-Type", "application/json")
		log.Warn("image retrieve: applications error", "client", clientAddress(r))

		_ = response.InternalError(errors.New("couldn't find matching update")).Render(w)

		return
	}

	// Include JSON files.
	assets = append(assets, "update.json", "update.sjson")

	// Check if we have compression in-transit.
	fileName := "update-" + req.Channel + "-" + version + ".tar.gz"
	setFileHeaders(w, r, fileName)

	writeToTar := func(tw *tar.Writer, asset string) error {
		updateFilePath := filepath.Join(os.Args[1], version, asset)

		updateFile, err := os.Open(updateFilePath)
		if err != nil {
			return fmt.Errorf("failed to read update file %q: %w", updateFilePath, err)
		}

		defer func() { _ = updateFile.Close() }()

		var rc io.Reader

		info, err := updateFile.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat update file %q: %w", updateFilePath, err)
		}

		rc = updateFile
		size := info.Size()
		fileName := asset

		//  Get the actual file size if compressed.
		if strings.HasSuffix(updateFilePath, ".gz") {
			fileName = strings.TrimSuffix(fileName, ".gz")

			rc, err = gzran.NewReader(updateFile)
			if err != nil {
				return fmt.Errorf("failed initial read of compressed update file %q: %w", updateFilePath, err)
			}

			size, err = io.Copy(io.Discard, rc)
			if err != nil {
				return fmt.Errorf("failed to read compressed update file %q: %w", updateFilePath, err)
			}

			err = updateFile.Close()
			if err != nil {
				return fmt.Errorf("failed to close update file %q: %w", updateFilePath, err)
			}

			updateFile, err = os.Open(updateFilePath)
			if err != nil {
				return fmt.Errorf("failed to read update file %q: %w", updateFilePath, err)
			}

			rc, err = gzran.NewReader(updateFile)
			if err != nil {
				return fmt.Errorf("failed to read compressed update file %q: %w", updateFilePath, err)
			}
		}

		err = tw.WriteHeader(&tar.Header{Name: fileName, Mode: 0o600, Size: size})
		if err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}

		for {
			_, err = io.CopyN(tw, rc, 4*1024*1024)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return fmt.Errorf("failed to write %q: %w", updateFilePath, err)
			}
		}

		return nil
	}

	gz := gzip.NewWriter(w)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	for _, asset := range assets {
		err := writeToTar(tw, asset)
		if err != nil {
			slog.Warn("image retrieve: bad update", "client", clientAddress(r), "err", err)
			w.Header().Set("Content-Type", "application/json")
			_ = response.InternalError(err).Render(w)

			return
		}
	}

	slog.Info("image retrieve: retrieved", "client", clientAddress(r))
}

func filterAssets(metaIndex apiupdate.Index, req apicustomizer.UpdateFilter) (string, []string, error) {
	assets := []string{}

	var highestVersion string

	highestVersionIndex := -1

	for i, update := range metaIndex.Updates {
		if !slices.Contains(update.Channels, req.Channel) {
			continue
		}

		// Match against the version if set.
		if req.Version != "" && update.Version == req.Version {
			highestVersionIndex = i

			break
		}

		if update.Version > highestVersion {
			highestVersion = update.Version
			highestVersionIndex = i

			continue
		}
	}

	if highestVersionIndex < 0 || (metaIndex.Updates[highestVersionIndex].Version != req.Version && req.Version != "") {
		return "", nil, errors.New("version not found")
	}

	for _, fileEntry := range metaIndex.Updates[highestVersionIndex].Files {
		if len(req.Architectures) > 0 && !slices.Contains(req.Architectures, fileEntry.Architecture) {
			continue
		}

		if len(req.Types) > 0 && !slices.Contains(req.Types, fileEntry.Type) {
			continue
		}

		if len(req.Components) > 0 && !slices.Contains(req.Components, fileEntry.Component) {
			continue
		}

		assets = append(assets, fileEntry.Filename)
	}

	return metaIndex.Updates[highestVersionIndex].Version, assets, nil
}

func parseIndex() (*apiupdate.Index, error) {
	// Find latest update.
	var metaIndex apiupdate.Index

	metaFile, err := os.Open(filepath.Join(os.Args[1], "index.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	defer func() { _ = metaFile.Close() }()

	err = json.NewDecoder(metaFile).Decode(&metaIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse index: %w", err)
	}

	return &metaIndex, nil
}

func setFileHeaders(w http.ResponseWriter, r *http.Request, fileName string) {
	compress := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
	if compress {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/octet-stream")

		fileName = strings.TrimSuffix(fileName, ".gz")
	} else {
		w.Header().Set("Content-Type", "application/gzip")
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	w.WriteHeader(http.StatusOK)
}

func buildImage(imageUUID string, fileType string, tempFile string, version string, assets []string) error {
	tempDir, err := os.MkdirTemp("/tmp", "rescue-"+imageUUID)
	if err != nil {
		return err
	}

	defer os.RemoveAll(tempDir)

	updateDir := filepath.Join(tempDir, "update")

	err = os.Mkdir(updateDir, 0o700)
	if err != nil {
		return err
	}

	totalSize := int64(0)

	for _, asset := range assets {
		dir := filepath.Dir(asset)
		if dir != "." {
			err := os.MkdirAll(filepath.Join(updateDir, dir), 0o700)
			if err != nil {
				return err
			}
		}

		updateFilePath := filepath.Join(os.Args[1], version, asset)
		targetFile := filepath.Join(updateDir, strings.TrimSuffix(asset, ".gz"))

		o, err := os.Create(targetFile)
		if err != nil {
			return fmt.Errorf("failed to create target file %q: %w", targetFile, err)
		}

		defer o.Close() //nolint:revive

		f, err := os.Open(updateFilePath)
		if err != nil {
			return fmt.Errorf("failed to open update file %q: %w", updateFilePath, err)
		}

		defer f.Close() //nolint:revive

		if strings.HasSuffix(updateFilePath, ".gz") {
			gz, err := gzip.NewReader(f)
			if err != nil {
				return fmt.Errorf("failed to open compressed file %q: %w", updateFilePath, err)
			}

			defer gz.Close() //nolint:revive

			n, err := io.Copy(o, gz) //nolint:gosec // This file is local to us.
			if err != nil {
				return fmt.Errorf("failed to copy files %q -> %q: %w", updateFilePath, targetFile, err)
			}

			totalSize += n
		} else {
			n, err := io.Copy(o, f)
			if err != nil {
				return fmt.Errorf("failed to copy files %q -> %q: %w", updateFilePath, targetFile, err)
			}

			totalSize += n
		}
	}

	if fileType == imageTypeISO {
		_, err := subprocess.RunCommand("mkisofs", "-V", "RESCUE_DATA", "-joliet-long", "-rock", "-o", tempFile, tempDir)
		if err != nil {
			return err
		}

		return nil
	}

	f, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", tempFile, err)
	}

	defer f.Close()

	// Use the total size plus the VFAT offset, plus an additional 10MiB of padding, rounded up to the nearest 512.
	paddedSize := (totalSize + (512 * 2048) + (10 * 1024 * 1024) + 511) / 512 * 512

	err = f.Truncate(paddedSize)
	if err != nil {
		return fmt.Errorf("failed to truncate file %q: %w", tempFile, err)
	}

	_, err = subprocess.RunCommand("sgdisk", "-n", "1", "-c", "1:RESCUE_DATA", tempFile)
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommand("mkfs.vfat", "-S", "512", "--offset=2048", tempFile)
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommand("mcopy", "-s", "-i", tempFile+"@@1048576", updateDir, "::/")
	if err != nil {
		return err
	}

	return nil
}

func writeSeed(writer io.Writer, seeds apicustomizer.ImagesPostSeeds) (int, error) {
	archiveContents := [][]string{}

	// Create applications yaml contents.
	if seeds.Applications != nil {
		yamlContents, err := yaml.Dump(seeds.Applications, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"applications.yaml", string(yamlContents)})
	}

	// Create incus yaml contents.
	if seeds.Incus != nil {
		yamlContents, err := yaml.Dump(seeds.Incus, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"incus.yaml", string(yamlContents)})
	}

	// Create operations-center yaml contents.
	if seeds.OperationsCenter != nil {
		yamlContents, err := yaml.Dump(seeds.OperationsCenter, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"operations-center.yaml", string(yamlContents)})
	}

	// Create migration-manager yaml contents.
	if seeds.MigrationManager != nil {
		yamlContents, err := yaml.Dump(seeds.MigrationManager, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"migration-manager.yaml", string(yamlContents)})
	}

	// Create install yaml contents.
	if seeds.Install != nil {
		yamlContents, err := yaml.Dump(seeds.Install, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"install.yaml", string(yamlContents)})
	}

	// Create network yaml contents.
	if seeds.Network != nil {
		yamlContents, err := yaml.Dump(seeds.Network, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"network.yaml", string(yamlContents)})
	}

	// Create provider yaml contents.
	if seeds.Provider != nil {
		yamlContents, err := yaml.Dump(seeds.Provider, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"provider.yaml", string(yamlContents)})
	}

	// Create update yaml contents.
	if seeds.Update != nil {
		yamlContents, err := yaml.Dump(seeds.Update, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"update.yaml", string(yamlContents)})
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
