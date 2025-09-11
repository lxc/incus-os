package rest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

func (*Server) apiDebug(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	endpoint, _ := url.JoinPath(getAPIRoot(r), "debug/log")

	_ = response.SyncResponse(true, []string{endpoint}).Render(w)
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
