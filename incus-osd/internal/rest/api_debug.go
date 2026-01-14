package rest

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/google/go-eventlog/tcg"
	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
)

// swagger:operation GET /1.0/debug debug debug_get
//
//	Get debug endpoints
//
//	Returns a list of debug endpoints (URLs).
//
//	These endpoints have no guarantee of API stability, and should not be used in normal day-to-day operations.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API endpoints
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: List of debug endpoints
//	          items:
//	            type: string
//	          example: ["/1.0/debug/log","/1.0/debug/tui"]
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

// swagger:operation GET /1.0/debug/log debug debug_get_log
//
//	Get systemd journal entries
//
//	Return systemd journal entries, optionally filtering by unit, boot number, and number of returned entries.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: unit
//	    description: Limit journal entries to the specified unit
//	    required: false
//	    type: string
//	  - in: query
//	    name: boot
//	    description: Limit journal entries to the specified boot number
//	    required: false
//	    type: integer
//	  - in: query
//	    name: entries
//	    description: Limit journal entries to the specified number of entries
//	    required: false
//	    type: integer
//	responses:
//	  "200":
//	    description: systemd journal entries
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: List of systemd journal entries
//	          items:
//	            type: object
//	          example: [{"MESSAGE":"2025-11-04 16:07:01 INFO System is ready release=202511041601","PRIORITY":"6","SYSLOG_FACILITY":"3","SYSLOG_IDENTIFIER":"incus-osd","_BOOT_ID":"800f36431cb84ddbacbff7fd5539d359","_CAP_EFFECTIVE":"1ffffffffff","_CMDLINE":"/usr/local/bin/incus-osd","_COMM":"incus-osd","_EXE":"/usr/local/bin/incus-osd","_GID":"0","_HOSTNAME":"af94e64e-1993-41b6-8f10-a8eebb828fce","_MACHINE_ID":"af94e64e199341b68f10a8eebb828fce","_PID":"688","_RUNTIME_SCOPE":"system","_SELINUX_CONTEXT":"unconfined\n","_STREAM_ID":"2cad567611724cb0ac38369beeff4921","_SYSTEMD_CGROUP":"/system.slice/incus-osd.service","_SYSTEMD_INVOCATION_ID":"8b2d8aabff73448dafab917f4eaaeacc","_SYSTEMD_SLICE":"system.slice","_SYSTEMD_UNIT":"incus-osd.service","_TRANSPORT":"stdout","_UID":"0","__CURSOR":"s=55e9886cc9024eb7ad4367e9061be6ce;i=7a6;b=800f36431cb84ddbacbff7fd5539d359;m=241064e;t=642c705aba083;x=e88fc1e4f70c128a","__MONOTONIC_TIMESTAMP":"37815886","__REALTIME_TIMESTAMP":"1762272421322883","__SEQNUM":"1958","__SEQNUM_ID":"55e9886cc9024eb7ad4367e9061be6ce"}]
//	  "500":
//	    $ref: "#/responses/InternalServerError"
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

	for line := range strings.SplitSeq(jsonOutput, "\n") {
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

// swagger:operation GET /1.0/debug/secureboot/event-log debug debug_get_secureboot_event_log
//
//	Get TPM event log
//
//	Get the system's TPM event log for PCRs 4, 7, & 11 and final values for PCRs 4 & 7. All digests are assumed to be SHA256.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: filtered TPM event log
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: object
//	          description: TPM event log information
//	          properties:
//	            event_log:
//	              description: Filtered TPM event log
//	              type: list
//	              example: [{"Index":7,"Type":2147483649,"Data":"Yd/ki8qT0hGqDQDgmAMrjAoAAAAAAAAAAQAAAAAAAABTAGUAYwB1AHIAZQBCAG8AbwB0AAE=","Digest":"zPxLsyiIo0W8iuraulUrYn2ZNIx2doGrMUH1sB5ApA4="}]
//	            pcr4:
//	              description: Final PCR4 value
//	              type: string
//	              example: 63d3f130685f6566dff2eaf957c99fc6bb6e0616eb5ecb8492f9102a6def81ca
//	            pcr7:
//	              description: Final PCR7 value
//	              type: string
//	              example: ab0565daf964cf56a89b220ce95df061884393691b035d11174e26196be062b0
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiDebugSecureBootEventLog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	eventLog, err := secureboot.ReadTPMEventLog()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	// Only return events for PCRs 4, 7, & 11.
	eventLog = slices.DeleteFunc(eventLog, func(e tcg.Event) bool {
		return e.Index != 4 && e.Index != 7 && e.Index != 11
	})

	pcr4Value, err := secureboot.ReadPCR("4")
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	pcr7Value, err := secureboot.ReadPCR("7")
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	type retStruct struct {
		EventLog []tcg.Event `json:"event_log"`
		PCR4     string      `json:"pcr4"`
		PCR7     string      `json:"pcr7"`
	}

	ret := retStruct{
		EventLog: eventLog,
		PCR4:     hex.EncodeToString(pcr4Value),
		PCR7:     hex.EncodeToString(pcr7Value),
	}

	_ = response.SyncResponse(true, ret).Render(w)
}

// swagger:operation POST /1.0/debug/secureboot/:update debug debug_post_secureboot_update
//
//	Apply Secure Boot updates
//
//	Apply a `gzip` compressed tar archive of Secure Boot variable updates.
//
//	Remember to properly set the `Content-Type: application/gzip` HTTP header.
//
//	---
//	consumes:
//	  - application/gzip
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: gzip tar archive
//	    description: Secure Boot updates to apply
//	    required: true
//	    schema:
//	      type: file
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiDebugSecureBootUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Determine Secure Boot state.
	sbEnabled, err := secureboot.Enabled()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	if !sbEnabled {
		_ = response.BadRequest(errors.New("cannot apply certificate update when Secure Boot is disabled")).Render(w)

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
