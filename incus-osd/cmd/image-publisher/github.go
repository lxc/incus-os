package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"

	ghapi "github.com/google/go-github/v72/github"
)

type github struct {
	client       *ghapi.Client
	organization string
	repository   string
}

func (p *github) downloadAsset(ctx context.Context, assetID int64, target string) (string, int64, error) {
	// Get a reader for the release asset.
	rc, _, err := p.client.Repositories.DownloadReleaseAsset(ctx, p.organization, p.repository, assetID, http.DefaultClient)
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
