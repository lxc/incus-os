package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
)

func (*Server) apiDebug(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	endpoint, _ := url.JoinPath(getAPIRoot(r), "debug")

	urls := []string{}

	for _, debug := range []string{"log", "tui"} {
		debugURL, _ := url.JoinPath(endpoint, debug)
		urls = append(urls, debugURL)
	}

	_ = response.SyncResponse(true, urls).Render(w)
}

func (*Server) apiDebugLog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	err := r.ParseForm()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	unitName := r.Form.Get("unit")
	bootNumber := r.Form.Get("boot")
	numEntries := r.Form.Get("entries")

	journalCmdArgs := []string{"-o", "json"}

	if unitName != "" {
		journalCmdArgs = append(journalCmdArgs, "-u", unitName)
	}

	if bootNumber != "" {
		journalCmdArgs = append(journalCmdArgs, "-b", bootNumber)
	} else {
		journalCmdArgs = append(journalCmdArgs, "-b", "0")
	}

	if numEntries != "" {
		journalCmdArgs = append(journalCmdArgs, "-n", numEntries)
	}

	jsonOutput, err := subprocess.RunCommandContext(r.Context(), "journalctl", journalCmdArgs...)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	jsonObj := []map[string]any{}

	for _, line := range strings.Split(jsonOutput, "\n") {
		if line == "" {
			continue
		}

		obj := map[string]any{}

		err = json.Unmarshal([]byte(line), &obj)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		jsonObj = append(jsonObj, obj)
	}

	_ = response.SyncResponse(true, jsonObj).Render(w)
}

func (*Server) apiDebugSecureBootUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Write the request body to a temporary file.
	f, err := os.CreateTemp("", "incus-os-sb-update")
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}
	defer os.Remove(f.Name())

	_, err = io.Copy(f, r.Body)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	err = f.Close()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	// Invoke the Secure Boot update process.
	_, err = secureboot.UpdateSecureBootCerts(r.Context(), f.Name())
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}
