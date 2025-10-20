package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/api"
)

const dateLayoutSecond = "2006/01/02 15:04:05 MST"

func doQuery(do func(remoteName string, req *http.Request) (*http.Response, error), remote string, method string, path string, inData any, outData io.Writer, etag string) (*api.Response, string, error) {
	var (
		req *http.Request
		err error
	)

	ctx := context.Background()

	// Get a new HTTP request setup
	if inData != nil {
		switch data := inData.(type) {
		case io.Reader:
			// Some data to be sent along with the request
			req, err = http.NewRequestWithContext(ctx, method, path, io.NopCloser(data))
			if err != nil {
				return nil, "", err
			}

			req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(data), nil }

			// Set the encoding accordingly
			req.Header.Set("Content-Type", "application/octet-stream")
		case string:
			// Some data to be sent along with the request
			// Use a reader since the request body needs to be seekable
			req, err = http.NewRequestWithContext(ctx, method, path, strings.NewReader(data))
			if err != nil {
				return nil, "", err
			}

			req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(data)), nil }

			// Set the encoding accordingly
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Length", strconv.Itoa(len(data)))
		default:
			// Encode the provided data
			buf := bytes.Buffer{}

			err := json.NewEncoder(&buf).Encode(data)
			if err != nil {
				return nil, "", err
			}

			// Some data to be sent along with the request
			// Use a reader since the request body needs to be seekable
			req, err = http.NewRequestWithContext(ctx, method, path, bytes.NewReader(buf.Bytes()))
			if err != nil {
				return nil, "", err
			}

			req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(buf.Bytes())), nil }

			// Set the encoding accordingly
			req.Header.Set("Content-Type", "application/json")
		}
	} else {
		// No data to be sent along with the request
		req, err = http.NewRequestWithContext(ctx, method, path, nil)
		if err != nil {
			return nil, "", err
		}
	}

	// Set the ETag
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}

	// Send the request
	resp, err := do(remote, req)
	if err != nil {
		return nil, "", err
	}

	// Handle direct download.
	if outData != nil {
		for {
			_, err = io.CopyN(outData, resp.Body, 4*1024*1024)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return nil, "", err
			}
		}

		return nil, "", nil
	}

	// Handle JSON responses.
	defer func() { _ = resp.Body.Close() }()

	// Decode the response
	decoder := json.NewDecoder(resp.Body)
	response := api.Response{}

	err = decoder.Decode(&response)
	if err != nil {
		// Check the return value for a cleaner error
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("failed to fetch %s: %s", resp.Request.URL.String(), resp.Status)
		}

		return nil, "", err
	}

	// Handle errors
	if response.Type == api.ErrorResponse {
		return &response, "", api.StatusErrorf(resp.StatusCode, "%v", response.Error)
	}

	return &response, resp.Header.Get("ETag"), nil
}

func parseRemote(in string) (_ string, _ string) {
	fields := strings.SplitN(in, ":", 2)
	if len(fields) < 2 {
		return "", fields[0]
	}

	return fields[0], fields[1]
}

func makeJsonable(data any) any {
	switch x := data.(type) {
	case map[any]any:
		newData := map[string]any{}
		for k, v := range x {
			kStr, ok := k.(string)
			if !ok {
				continue
			}

			newData[kStr] = makeJsonable(v)
		}

		return newData
	case []any:
		for i, v := range x {
			x[i] = makeJsonable(v)
		}
	}

	return data
}
