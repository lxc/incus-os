package secureboot

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"
)

// TPMStatus returns basic information about the status of the TPM.
func TPMStatus() string {
	eventLog, err := readTMPEventLog()
	if err != nil {
		return err.Error()
	}

	err = validateUntrustedTPMEventLog(eventLog)
	if err != nil {
		return err.Error()
	}

	computedPCR, err := computeNewPCR7Value(eventLog)
	if err != nil {
		return err.Error()
	}

	actualPCR, err := readPCR7()
	if err != nil {
		return err.Error()
	}

	if !bytes.Equal(computedPCR, actualPCR) {
		return TPMPCRMismatch
	}

	return "ok"
}

// readTMPEventLog reads the raw TPM measurements and returns a parsed array of Events with SHA256 hashes.
func readTMPEventLog() ([]tcg.Event, error) {
	rawLog, err := os.Open("/sys/kernel/security/tpm0/binary_bios_measurements")
	if err != nil {
		return nil, err
	}
	defer rawLog.Close()

	buf, err := io.ReadAll(rawLog)
	if err != nil {
		return nil, err
	}

	log, err := tcg.ParseEventLog(buf, tcg.ParseOpts{})
	if err != nil {
		return nil, err
	}

	return log.Events(register.HashSHA256), nil
}

// validateUntrustedTPMEventLog takes an untrusted TPM event log and verifies if its values
// match what is currently reported by the TPM.
func validateUntrustedTPMEventLog(eventLog []tcg.Event) error {
	var err error

	// Playback the log and compute the resulting PCR7 value.
	untrustedPCR7Digest := make([]byte, 32)

	for _, e := range eventLog {
		if e.Index == 7 { // We only care about PCR7.
			untrustedPCR7Digest, err = extendPCRValue(untrustedPCR7Digest, e.ReplayedDigest(), false)
			if err != nil {
				return err
			}
		}
	}

	// Get the current PCR7 value from the TPM.
	actualPCR7, err := readPCR7()
	if err != nil {
		return err
	}

	if !bytes.Equal(actualPCR7, untrustedPCR7Digest) {
		return fmt.Errorf("computed PCR7 (%x) doesn't match actual value (%x)", untrustedPCR7Digest, actualPCR7)
	}

	return nil
}
