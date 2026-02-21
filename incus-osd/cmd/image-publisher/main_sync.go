package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	ghapi "github.com/google/go-github/v72/github"
	"github.com/lxc/incus/v6/shared/osarch"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
)

type cmdSync struct {
	global *cmdGlobal
}

func (c *cmdSync) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "sync <path>"
	cmd.Short = "Imports new images and cleans up the tree"
	cmd.Long = formatSection("Description",
		`Imports new images and cleans up the tree

This will connect to Github to retrieve any new image that's missing
locally, then import them into the default channel (typically "testing")
and then cleans up any extra image based on retention policy.
`)
	cmd.RunE = c.run

	return cmd
}

func (c *cmdSync) run(cmd *cobra.Command, args []string) error {
	ctx := context.TODO()

	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	targetPath := args[0]

	err = os.MkdirAll(targetPath, 0o755)
	if err != nil {
		return err
	}

	// Config (optional).
	updateChannel := os.Getenv("UPDATE_CHANNEL")
	if updateChannel == "" {
		updateChannel = "testing"
	}

	updateOrigin := os.Getenv("UPDATE_ORIGIN")
	if updateOrigin == "" {
		updateOrigin = "linuxcontainers.org"
	}

	updateSeverity := os.Getenv("UPDATE_SEVERITY")
	if updateSeverity == "" {
		updateSeverity = "none"
	}

	// Get the latest image info.
	releaseName, releaseURLs, err := getLatestRelease(ctx)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "Found latest image", "version", releaseName)

	// Prepare the update.json.
	metaUpdate := apiupdate.Update{
		Format: "1.0",

		Channels:    []string{updateChannel},
		Files:       []apiupdate.UpdateFile{},
		Origin:      updateOrigin,
		PublishedAt: time.Now().UTC(),
		Severity:    apiupdate.UpdateSeverity(updateSeverity),
		Version:     releaseName,
	}

	// Create the release folder.
	err = os.Mkdir(filepath.Join(targetPath, releaseName), 0o755)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			slog.InfoContext(ctx, "Latest release already imported")

			return nil
		}

		return err
	}

	// Get the image files.
	var muDownload sync.Mutex

	g := new(errgroup.Group)

	for imageArch, imageURL := range releaseURLs {
		// Convert the architecture name.
		archID, err := osarch.ArchitectureID(imageArch)
		if err != nil {
			return err
		}

		archName, err := osarch.ArchitectureName(archID)
		if err != nil {
			return err
		}

		// Download the image.
		targetPath := filepath.Join(targetPath, releaseName)

		g.Go(func() error {
			files, err := c.downloadImage(ctx, archName, imageURL, targetPath)
			if err != nil {
				return err
			}

			muDownload.Lock()

			metaUpdate.Files = append(metaUpdate.Files, files...)

			muDownload.Unlock()

			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return err
	}

	// Include the SecureBoot update (if present).
	updateSecureboot := os.Getenv("UPDATE_SECUREBOOT")
	if updateSecureboot != "" {
		// Open the update tarball.
		f, err := os.Open(updateSecureboot)
		if err != nil {
			return err
		}

		defer func() { _ = f.Close() }()

		// Setup a hashing reader.
		h := sha256.New()
		r := io.TeeReader(f, h)

		// Create the target file.
		w, err := os.Create(filepath.Join(targetPath, releaseName, filepath.Base(updateSecureboot)))
		if err != nil {
			return err
		}

		defer func() { _ = w.Close() }()

		// Copy the content.
		var size int64

		for {
			n, err := io.CopyN(w, r, 4*1024*1024)
			size += n

			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return err
			}
		}

		// Add the file to the image.
		metaUpdate.Files = append(metaUpdate.Files, apiupdate.UpdateFile{
			Component: apiupdate.UpdateFileComponentOS,
			Filename:  filepath.Base(updateSecureboot),
			Sha256:    hex.EncodeToString(h.Sum(nil)),
			Size:      size,
			Type:      apiupdate.UpdateFileTypeUpdateSecureboot,
		})
	}

	// Generate changelog.
	err = generateChangelog(&metaUpdate, metaUpdate.Channels[0], filepath.Join(targetPath, releaseName))
	if err != nil {
		return err
	}

	// Write the update metadata.
	wr, err := os.Create(filepath.Join(targetPath, releaseName, "update.json"))
	if err != nil {
		return err
	}

	defer func() { _ = wr.Close() }()

	err = json.NewEncoder(wr).Encode(metaUpdate)
	if err != nil {
		return err
	}

	err = wr.Close()
	if err != nil {
		return err
	}

	err = sign(ctx, filepath.Join(targetPath, releaseName, "update.json"), filepath.Join(targetPath, releaseName, "update.sjson"))
	if err != nil {
		return err
	}

	// Re-generate the index.
	err = generateIndex(ctx, args[0])
	if err != nil {
		return err
	}

	return nil
}

func (*cmdSync) downloadImage(ctx context.Context, archName string, releaseURL *url.URL, targetPath string) ([]apiupdate.UpdateFile, error) {
	files := []apiupdate.UpdateFile{}

	slog.InfoContext(ctx, "Downloading image", "arch", archName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response from server: %d", resp.StatusCode)
	}

	tempImage, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}

	defer func() { _ = os.Remove(tempImage.Name()) }()

	var size int64

	for {
		n, err := io.CopyN(tempImage, resp.Body, 4*1024*1024)
		size += n

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}
	}

	// Parse the image file.
	zr, err := zip.OpenReader(tempImage.Name())
	if err != nil {
		return nil, err
	}

	for _, f := range zr.File {
		assetName := f.Name

		// Check if file should be imported.
		var (
			assetComponent apiupdate.UpdateFileComponent
			assetType      apiupdate.UpdateFileType
		)

		switch {
		case assetName == "debug.raw.gz":
			assetComponent = apiupdate.UpdateFileComponentDebug
			assetType = apiupdate.UpdateFileTypeApplication
		case assetName == "gpu-support.raw.gz":
			assetComponent = apiupdate.UpdateFileComponentGPUSupport
			assetType = apiupdate.UpdateFileTypeApplication
		case assetName == "incus.raw.gz":
			assetComponent = apiupdate.UpdateFileComponentIncus
			assetType = apiupdate.UpdateFileTypeApplication
		case assetName == "incus-ceph.raw.gz":
			assetComponent = apiupdate.UpdateFileComponentIncusCeph
			assetType = apiupdate.UpdateFileTypeApplication
		case assetName == "incus-linstor.raw.gz":
			assetComponent = apiupdate.UpdateFileComponentIncusLinstor
			assetType = apiupdate.UpdateFileTypeApplication
		case assetName == "migration-manager.raw.gz":
			assetComponent = apiupdate.UpdateFileComponentMigrationManager
			assetType = apiupdate.UpdateFileTypeApplication
		case assetName == "openfga.raw.gz":
			assetComponent = apiupdate.UpdateFileComponentOpenFGA
			assetType = apiupdate.UpdateFileTypeApplication
		case assetName == "operations-center.raw.gz":
			assetComponent = apiupdate.UpdateFileComponentOperationsCenter
			assetType = apiupdate.UpdateFileTypeApplication
		case strings.HasSuffix(assetName, ".efi.gz"):
			assetComponent = apiupdate.UpdateFileComponentOS
			assetType = apiupdate.UpdateFileTypeUpdateEFI
		case strings.HasSuffix(assetName, ".img.gz"):
			assetComponent = apiupdate.UpdateFileComponentOS
			assetType = apiupdate.UpdateFileTypeImageRaw
		case strings.HasSuffix(assetName, ".iso.gz"):
			assetComponent = apiupdate.UpdateFileComponentOS
			assetType = apiupdate.UpdateFileTypeImageISO
		case strings.Contains(assetName, ".usr-x86-64-verity."), strings.Contains(assetName, ".usr-arm64-verity."):
			assetComponent = apiupdate.UpdateFileComponentOS
			assetType = apiupdate.UpdateFileTypeUpdateUsrVerity
		case strings.Contains(assetName, ".usr-x86-64-verity-sig."), strings.Contains(assetName, ".usr-arm64-verity-sig."):
			assetComponent = apiupdate.UpdateFileComponentOS
			assetType = apiupdate.UpdateFileTypeUpdateUsrVeritySignature
		case strings.Contains(assetName, ".usr-x86-64."), strings.Contains(assetName, ".usr-arm64."):
			assetComponent = apiupdate.UpdateFileComponentOS
			assetType = apiupdate.UpdateFileTypeUpdateUsr
		case strings.HasSuffix(assetName, "debug.manifest.json.gz"):
			assetComponent = apiupdate.UpdateFileComponentDebug
			assetType = apiupdate.UpdateFileTypeImageManifest
		case strings.HasSuffix(assetName, "incus.manifest.json.gz"):
			assetComponent = apiupdate.UpdateFileComponentIncus
			assetType = apiupdate.UpdateFileTypeImageManifest
		case strings.HasSuffix(assetName, "migration-manager.manifest.json.gz"):
			assetComponent = apiupdate.UpdateFileComponentMigrationManager
			assetType = apiupdate.UpdateFileTypeImageManifest
		case strings.HasSuffix(assetName, "openfga.manifest.json.gz"):
			assetComponent = apiupdate.UpdateFileComponentOpenFGA
			assetType = apiupdate.UpdateFileTypeImageManifest
		case strings.HasSuffix(assetName, "operations-center.manifest.json.gz"):
			assetComponent = apiupdate.UpdateFileComponentOperationsCenter
			assetType = apiupdate.UpdateFileTypeImageManifest
		case strings.HasSuffix(assetName, ".manifest.json.gz"):
			assetComponent = apiupdate.UpdateFileComponentOS
			assetType = apiupdate.UpdateFileTypeImageManifest
		default:
			continue
		}

		// Create the per-architecture path.
		err = os.MkdirAll(filepath.Join(targetPath, archName), 0o755)
		if err != nil {
			return nil, err
		}

		// Extract the file.
		slog.InfoContext(ctx, "Extracting", "name", assetName, "arch", archName)

		assetHash, assetSize, err := extractFile(f, filepath.Join(targetPath, archName, assetName)) //nolint:gosec
		if err != nil {
			return nil, err
		}

		// Add to the index.
		files = append(files, apiupdate.UpdateFile{
			Architecture: apiupdate.UpdateFileArchitecture(archName),
			Component:    assetComponent,
			Filename:     filepath.Join(archName, assetName), //nolint:gosec
			Sha256:       assetHash,
			Size:         assetSize,
			Type:         assetType,
		})
	}

	return files, nil
}

func extractFile(f *zip.File, target string) (string, int64, error) {
	// Open the file.
	rc, err := f.Open()
	if err != nil {
		return "", 0, err
	}

	defer rc.Close()

	// Create the target path.
	// #nosec G304
	fd, err := os.Create(target)
	if err != nil {
		return "", 0, err
	}

	defer fd.Close()

	// Hashing logic.
	hash256 := sha256.New()

	// Target writer.
	wr := io.MultiWriter(fd, hash256)

	// Read from the decompressor in chunks to avoid excessive memory consumption.
	var size int64

	for {
		n, err := io.CopyN(wr, rc, 4*1024*1024)
		size += n

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return "", 0, err
		}
	}

	return hex.EncodeToString(hash256.Sum(nil)), size, nil
}

func getLatestRelease(ctx context.Context) (string, map[string]*url.URL, error) {
	// Config (optional).
	ghOrganization := os.Getenv("GH_ORGANIZATION")
	if ghOrganization == "" {
		ghOrganization = "lxc"
	}

	ghRepository := os.Getenv("GH_REPOSITORY")
	if ghRepository == "" {
		ghRepository = "incus-os"
	}

	// Setup client.
	client := ghapi.NewClient(nil)

	if os.Getenv("GH_TOKEN") != "" {
		client = client.WithAuthToken(os.Getenv("GH_TOKEN"))
	}

	// Get the latest build.
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, ghOrganization, ghRepository, &ghapi.ListWorkflowRunsOptions{
		Event:               "push",
		Status:              "completed",
		ExcludePullRequests: true,
	})
	if err != nil {
		return "", nil, err
	}

	var latestRun *ghapi.WorkflowRun

	for _, run := range runs.WorkflowRuns {
		if *run.Repository.FullName != ghOrganization+"/"+ghRepository {
			continue
		}

		if *run.Conclusion != "success" {
			continue
		}

		if *run.Name != "Build" {
			continue
		}

		latestRun = run

		break
	}

	if latestRun == nil {
		return "", nil, errors.New("couldn't find any matching run")
	}

	releaseName := *latestRun.HeadBranch

	// Get the image file.
	artifacts, _, err := client.Actions.ListWorkflowRunArtifacts(ctx, ghOrganization, ghRepository, *latestRun.ID, nil)
	if err != nil {
		return "", nil, err
	}

	images := map[string]*url.URL{}

	for _, artifact := range artifacts.Artifacts {
		if !strings.HasPrefix(*artifact.Name, "image-") {
			continue
		}

		fields := strings.SplitN(*artifact.Name, "-", 2)
		if len(fields) != 2 {
			continue
		}

		_, ok := images[fields[1]]
		if ok {
			continue
		}

		u, _, err := client.Actions.DownloadArtifact(ctx, ghOrganization, ghRepository, *artifact.ID, 10)
		if err != nil {
			return "", nil, err
		}

		images[fields[1]] = u
	}

	return releaseName, images, nil
}
