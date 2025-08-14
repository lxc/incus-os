// Package main is used for the image publisher.
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
	"time"

	ghapi "github.com/google/go-github/v72/github"
	"github.com/lxc/incus/v6/shared/subprocess"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
)

func main() {
	err := do(context.TODO())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func do(ctx context.Context) error {
	// Arguments.
	if len(os.Args) != 2 {
		return errors.New("missing target path")
	}

	targetPath := os.Args[1]
	err := os.MkdirAll(targetPath, 0755)
	if err != nil {
		return err
	}

	// Config (optional).
	updateChannel := os.Getenv("UPDATE_CHANNEL")
	if updateChannel == "" {
		updateChannel = "daily"
	}

	updateOrigin := os.Getenv("UPDATE_ORIGIN")
	if updateOrigin == "" {
		updateOrigin = "linuxcontainers.org"
	}

	updateSeverity := os.Getenv("UPDATE_SEVERITY")
	if updateSeverity == "" {
		updateSeverity = "none"
	}

	// Setup signer.
	sign := func(src string, dst string) error {
		if os.Getenv("SIG_KEY") == "" || os.Getenv("SIG_CERTIFICATE") == "" || os.Getenv("SIG_CHAIN") == "" {
			return nil
		}

		// Generate an SMIME signature.
		_, err := subprocess.RunCommandContext(ctx, "openssl", "smime", "-text", "-sign", "-signer", os.Getenv("SIG_CERTIFICATE"), "-inkey", os.Getenv("SIG_KEY"), "-in", src, "-out", dst, "-certfile", os.Getenv("SIG_CHAIN"))
		if err != nil {
			return err
		}

		return nil
	}

	// Get the latest image info.
	releaseName, releaseURL, err := getLatestRelease(ctx)
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
		PublishedAt: time.Now(),
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

	// Get the image file.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL.String(), nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response from server: %d", resp.StatusCode)
	}

	tempImage, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}

	defer func() { _ = os.Remove(tempImage.Name()) }()

	var size int64

	for {
		n, err := io.CopyN(tempImage, resp.Body, 4*1024*1024)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		size += n
	}

	slog.InfoContext(ctx, "Downloaded the image", "size", size)

	// Parse the image file.
	zr, err := zip.OpenReader(tempImage.Name())
	if err != nil {
		return err
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
		case assetName == "incus.raw.gz":
			assetComponent = apiupdate.UpdateFileComponentIncus
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
		case strings.Contains(assetName, ".usr-x86-64-verity."):
			assetComponent = apiupdate.UpdateFileComponentOS
			assetType = apiupdate.UpdateFileTypeUpdateUsrVerity
		case strings.Contains(assetName, ".usr-x86-64-verity-sig."):
			assetComponent = apiupdate.UpdateFileComponentOS
			assetType = apiupdate.UpdateFileTypeUpdateUsrVeritySignature
		case strings.Contains(assetName, ".usr-x86-64."):
			assetComponent = apiupdate.UpdateFileComponentOS
			assetType = apiupdate.UpdateFileTypeUpdateUsr
		default:
			continue
		}

		// Extract the file.
		assetHash, assetSize, err := extractFile(f, filepath.Join(targetPath, releaseName, assetName)) //nolint:gosec
		if err != nil {
			return err
		}

		// Add to the index.
		metaUpdate.Files = append(metaUpdate.Files, apiupdate.UpdateFile{
			Architecture: "x86_64",
			Component:    assetComponent,
			Filename:     assetName,
			Sha256:       assetHash,
			Size:         assetSize,
			Type:         assetType,
		})

		slog.InfoContext(ctx, "Extracted", "name", assetName, "hash", assetHash, "size", assetSize)
	}

	// Write the update metadata.
	wr, err := os.Create(filepath.Join(targetPath, releaseName, "update.json")) //nolint:gosec
	if err != nil {
		return err
	}

	defer func() { _ = wr.Close() }()

	err = json.NewEncoder(wr).Encode(metaUpdate)
	if err != nil {
		return err
	}

	err = sign(filepath.Join(targetPath, releaseName, "update.json"), filepath.Join(targetPath, releaseName, "update.sjson"))
	if err != nil {
		return err
	}

	// Write the index metadata.
	metaIndex := apiupdate.Index{
		Format:  "1.0",
		Updates: []apiupdate.UpdateFull{{Update: metaUpdate, URL: "/" + metaUpdate.Version}},
	}

	wr, err = os.Create(filepath.Join(targetPath, "index.json")) //nolint:gosec
	if err != nil {
		return err
	}

	defer func() { _ = wr.Close() }()

	err = json.NewEncoder(wr).Encode(metaIndex)
	if err != nil {
		return err
	}

	err = sign(filepath.Join(targetPath, "index.json"), filepath.Join(targetPath, "index.sjson"))
	if err != nil {
		return err
	}

	return nil
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

func getLatestRelease(ctx context.Context) (string, *url.URL, error) {
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

	var imageArtifact *ghapi.Artifact

	for _, artifact := range artifacts.Artifacts {
		if *artifact.Name != "Image" {
			continue
		}

		imageArtifact = artifact

		break
	}

	u, _, err := client.Actions.DownloadArtifact(ctx, ghOrganization, ghRepository, *imageArtifact.ID, 10)
	if err != nil {
		return "", nil, err
	}

	return releaseName, u, nil
}
