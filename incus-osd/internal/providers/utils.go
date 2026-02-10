package providers

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func downloadAsset(ctx context.Context, client *http.Client, assetURL string, expectedSHA256 string, target string, progressFunc func(float64)) error {
	// Remove the target file, if it exists. If we don't, truncating the existing file causes spurious
	// kernel log messages about verity device-mapper corrupted data blocks for sysext images.
	err := os.Remove(target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Prepare the request.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return errors.New("unable to create http request: " + err.Error())
	}

	// Get a reader for the release asset.
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("unable to get http response: " + err.Error())
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("unexpected HTTP status: " + resp.Status)
	}

	// Setup a sha256 hasher.
	h := sha256.New()

	// Setup the main reader.
	tr := io.TeeReader(resp.Body, h)

	// Setup a gzip reader to decompress during streaming.
	body, err := gzip.NewReader(tr)
	if err != nil {
		return errors.New("gzip error reading body: " + err.Error())
	}

	defer body.Close()

	// Create the target path.
	// #nosec G304
	fd, err := os.Create(target)
	if err != nil {
		return err
	}

	defer fd.Close()

	// Read from the decompressor in chunks to avoid excessive memory consumption.
	count := int64(0)

	for {
		_, err = io.CopyN(fd, body, 4*1024*1024)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return errors.New("io.CopyN() error: " + err.Error())
		}

		// Update progress every 24MiB.
		if progressFunc != nil && count%6 == 0 {
			progressFunc(float64(count*4*1024*1024) / float64(resp.ContentLength))
		}

		count++
	}

	// Check the hash.
	if expectedSHA256 != "" && expectedSHA256 != hex.EncodeToString(h.Sum(nil)) {
		return errors.New("sha256 mismatch for file " + target)
	}

	return nil
}

// tryRequest attempts the request multiple times over 5s.
func tryRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	var err error

	for range 5 {
		var resp *http.Response

		resp, err = client.Do(req)
		if err == nil {
			return resp, nil
		}

		time.Sleep(time.Second)
	}

	return nil, fmt.Errorf("http request timed out after five seconds: %w", err)
}
