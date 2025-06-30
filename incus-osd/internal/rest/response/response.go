package response

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lxc/incus/v6/shared/api"
)

// Response represents an API response.
type Response interface {
	Render(w http.ResponseWriter) error
	String() string
	Code() int
}

// Sync response.
type syncResponse struct {
	success   bool
	etag      any
	metadata  any
	location  string
	code      int
	headers   map[string]string
	plaintext bool
	compress  bool
}

// EmptySyncResponse represents an empty syncResponse.
var EmptySyncResponse = &syncResponse{success: true, metadata: make(map[string]any)}

// SyncResponse returns a new syncResponse with the success and metadata fields
// set to the provided values.
func SyncResponse(success bool, metadata any) Response {
	return &syncResponse{success: success, metadata: metadata}
}

// SyncResponseETag returns a new syncResponse with an etag.
func SyncResponseETag(success bool, metadata any, etag any) Response {
	return &syncResponse{success: success, metadata: metadata, etag: etag}
}

// SyncResponseLocation returns a new syncResponse with a location.
func SyncResponseLocation(success bool, metadata any, location string) Response {
	return &syncResponse{success: success, metadata: metadata, location: location}
}

// SyncResponseRedirect returns a new syncResponse with a location, indicating
// a permanent redirect.
func SyncResponseRedirect(address string) Response {
	return &syncResponse{success: true, location: address, code: http.StatusPermanentRedirect}
}

// SyncResponseHeaders returns a new syncResponse with headers.
func SyncResponseHeaders(success bool, metadata any, headers map[string]string) Response {
	return &syncResponse{success: success, metadata: metadata, headers: headers}
}

// SyncResponsePlain return a new syncResponse with plaintext.
func SyncResponsePlain(success bool, compress bool, metadata string) Response {
	return &syncResponse{success: success, metadata: metadata, plaintext: true, compress: compress}
}

func (r *syncResponse) Render(w http.ResponseWriter) error {
	// Set an appropriate ETag header
	if r.etag != nil {
		etag, err := etagHash(r.etag)
		if err == nil {
			w.Header().Set("ETag", fmt.Sprintf("\"%s\"", etag))
		}
	}

	if r.headers != nil {
		for h, v := range r.headers {
			w.Header().Set(h, v)
		}
	}

	if r.location != "" {
		w.Header().Set("Location", r.location)

		if r.code == 0 {
			r.code = 201
		}
	}

	// Handle plain text headers.
	if r.plaintext {
		w.Header().Set("Content-Type", "text/plain")
	}

	// Handle compression.
	if r.compress {
		w.Header().Set("Content-Encoding", "gzip")
	}

	// Write header and status code.
	if r.code == 0 {
		r.code = http.StatusOK
	}

	if w.Header().Get("Connection") != "keep-alive" {
		w.WriteHeader(r.code)
	}

	// Prepare the JSON response
	status := api.Success
	if !r.success {
		status = api.Failure

		// If the metadata is an error, consider the response a SmartError
		// to propagate the data and preserve the status code.
		err, ok := r.metadata.(error)
		if ok {
			return InternalError(err).Render(w)
		}
	}

	// Handle plain text responses.
	if r.plaintext {
		if r.metadata == nil {
			return nil
		}

		metadataValue, ok := r.metadata.(string)
		if !ok {
			return nil
		}

		if r.compress {
			comp := gzip.NewWriter(w)
			defer comp.Close()

			_, err := comp.Write([]byte(metadataValue))
			if err != nil {
				return err
			}

			return nil
		}

		_, err := w.Write([]byte(metadataValue))
		if err != nil {
			return err
		}

		return nil
	}

	// Handle JSON responses.
	resp := api.ResponseRaw{
		Type:       api.SyncResponse,
		Status:     status.String(),
		StatusCode: int(status),
		Metadata:   r.metadata,
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	err := enc.Encode(resp)
	if err != nil {
		return err
	}

	return nil
}

func (r *syncResponse) String() string {
	if r.success {
		return "success"
	}

	return "failure"
}

// Code returns the HTTP code.
func (r *syncResponse) Code() int {
	return r.code
}

// Error response.
type errorResponse struct {
	code int    // Code to return in both the HTTP header and Code field of the response body.
	msg  string // Message to return in the Error field of the response body.
}

// ErrorResponse returns an error response with the given code and msg.
func ErrorResponse(code int, msg string) Response {
	return &errorResponse{code, msg}
}

// BadRequest returns a bad request response (400) with the given error.
func BadRequest(err error) Response {
	return &errorResponse{http.StatusBadRequest, err.Error()}
}

// Conflict returns a conflict response (409) with the given error.
func Conflict(err error) Response {
	message := "already exists"
	if err != nil {
		message = err.Error()
	}

	return &errorResponse{http.StatusConflict, message}
}

// Forbidden returns a forbidden response (403) with the given error.
func Forbidden(err error) Response {
	message := "not authorized"
	if err != nil {
		message = err.Error()
	}

	return &errorResponse{http.StatusForbidden, message}
}

// InternalError returns an internal error response (500) with the given error.
func InternalError(err error) Response {
	return &errorResponse{http.StatusInternalServerError, err.Error()}
}

// NotFound returns a not found response (404) with the given error.
func NotFound(err error) Response {
	message := "not found"
	if err != nil {
		message = err.Error()
	}

	return &errorResponse{http.StatusNotFound, message}
}

// NotImplemented returns a not implemented response (501) with the given error.
func NotImplemented(err error) Response {
	message := "not implemented"
	if err != nil {
		message = err.Error()
	}

	return &errorResponse{http.StatusNotImplemented, message}
}

// PreconditionFailed returns a precondition failed response (412) with the
// given error.
func PreconditionFailed(err error) Response {
	return &errorResponse{http.StatusPreconditionFailed, err.Error()}
}

// Unavailable return an unavailable response (503) with the given error.
func Unavailable(err error) Response {
	message := "unavailable"
	if err != nil {
		message = err.Error()
	}

	return &errorResponse{http.StatusServiceUnavailable, message}
}

func (r *errorResponse) String() string {
	return r.msg
}

// Code returns the HTTP code.
func (r *errorResponse) Code() int {
	return r.code
}

func (r *errorResponse) Render(w http.ResponseWriter) error {
	resp := api.ResponseRaw{
		Type:  api.ErrorResponse,
		Error: r.msg,
		Code:  r.code, // Set the error code in the Code field of the response body.
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if w.Header().Get("Connection") != "keep-alive" {
		w.WriteHeader(r.code) // Set the error code in the HTTP header response.
	}

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		return err
	}

	return nil
}

// FileResponseEntry represents a file response entry.
type FileResponseEntry struct {
	// Required.
	Identifier string
	Filename   string

	// Read from a filesystem path.
	Path string

	// Read from a file.
	File         io.ReadSeeker
	FileSize     int64
	FileModified time.Time

	// Optional.
	Cleanup func()
}
type manualResponse struct {
	hook func(w http.ResponseWriter) error
}

// ManualResponse creates a new manual response responder.
func ManualResponse(hook func(w http.ResponseWriter) error) Response {
	return &manualResponse{hook: hook}
}

func (r *manualResponse) Render(w http.ResponseWriter) error {
	return r.hook(w)
}

func (*manualResponse) String() string {
	return "unknown"
}

// Code returns the HTTP code.
func (*manualResponse) Code() int {
	return http.StatusNotImplemented
}

// Unauthorized return an unauthorized response (401) with the given error.
func Unauthorized(err error) Response {
	message := "unauthorized"
	if err != nil {
		message = err.Error()
	}

	return &errorResponse{http.StatusUnauthorized, message}
}
