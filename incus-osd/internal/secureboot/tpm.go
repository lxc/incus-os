package secureboot

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"
)

// TPMStatus returns basic information about the status of the TPM.
func TPMStatus() string {
	eventLog, err := GetValidatedTPMEventLog()
	if err != nil {
		return err.Error()
	}

	computedPCR, err := computeNewPCR7Value(eventLog)
	if err != nil {
		return err.Error()
	}

	actualPCR, err := ReadPCR("7")
	if err != nil {
		return err.Error()
	}

	if !bytes.Equal(computedPCR, actualPCR) {
		return TPMPCRMismatch
	}

	if GetSWTPMInUse() {
		// We have a swtpm TPM in a good state.
		return "swtpm"
	}

	// We have a physical TPM in a good state.
	return "ok"
}

// GetSWTPMInUse returns a boolean indicating if a swtpm-backed TPM is running.
func GetSWTPMInUse() bool {
	// If a kernel TPM event log exists, that means we have a real TPM.
	_, err := os.Stat("/sys/kernel/security/tpm0/binary_bios_measurements")
	if err == nil {
		return false
	}

	// If a swtpm state directory exists, the swtpm service should be running.
	_, err = os.Stat("/boot/swtpm/")
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return false
	}

	return true
}

// ReadTPMEventLog reads the raw TPM measurements and returns a parsed array of Events with SHA256 hashes.
// The log entries are NOT verified by this function.
func ReadTPMEventLog() ([]tcg.Event, error) {
	var buf []byte

	var err error

	rawLog, err := os.Open("/sys/kernel/security/tpm0/binary_bios_measurements")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		// Fallback to a synthesized TPM event log for swtpm.
		buf, err = SynthesizeTPMEventLog()
		if err != nil {
			return nil, err
		}
	} else {
		defer rawLog.Close()

		buf, err = io.ReadAll(rawLog)
		if err != nil {
			return nil, err
		}
	}

	log, err := tcg.ParseEventLog(buf, tcg.ParseOpts{})
	if err != nil {
		return nil, err
	}

	return log.Events(register.HashSHA256), nil
}

// GetValidatedTPMEventLog returns a TPM event log that has had its PCR 4 & 7 values
// validated against what is reported by the TPM.
func GetValidatedTPMEventLog() ([]tcg.Event, error) {
	eventLog, err := ReadTPMEventLog()
	if err != nil {
		return nil, err
	}

	// Playback the log and compute the resulting PCR4 and PCR7 values.
	untrustedPCR4Digest := make([]byte, 32)
	untrustedPCR7Digest := make([]byte, 32)

	for _, e := range eventLog {
		switch e.Index {
		case 4:
			untrustedPCR4Digest, err = extendPCRValue(untrustedPCR4Digest, e.ReplayedDigest(), false)
			if err != nil {
				return nil, err
			}
		case 7:
			untrustedPCR7Digest, err = extendPCRValue(untrustedPCR7Digest, e.ReplayedDigest(), false)
			if err != nil {
				return nil, err
			}
		default: // Ignore all other PCRs.
		}
	}

	// Get the current PCR4 and PCR7 values from the TPM.
	actualPCR4, err := ReadPCR("4")
	if err != nil {
		return nil, err
	}

	actualPCR7, err := ReadPCR("7")
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(actualPCR4, untrustedPCR4Digest) {
		return nil, fmt.Errorf("computed PCR4 (%x) doesn't match actual value (%x)", untrustedPCR4Digest, actualPCR4)
	}

	if !bytes.Equal(actualPCR7, untrustedPCR7Digest) {
		return nil, fmt.Errorf("computed PCR7 (%x) doesn't match actual value (%x)", untrustedPCR7Digest, actualPCR7)
	}

	return eventLog, nil
}
