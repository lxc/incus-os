// Package main provides IncusOS utility commands that run in the initrd.
package main

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"
	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"

	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
)

func main() {
	// Silence all logging output.
	slog.SetDefault(slog.New(slog.DiscardHandler))

	var err error

	if len(os.Args) == 1 {
		err = fmt.Errorf("usage: %s <cmd>", os.Args[0])
	} else {
		switch os.Args[1] {
		case "measure-pcrs":
			err = measurePCRs()
		case "validate-pe-binaries":
			err = secureboot.ValidatePEBinaries()
		default:
			err = fmt.Errorf("unsupported action '%s'", os.Args[1])
		}
	}

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())

		os.Exit(1)
	}
}

// measurePCRs calculates and sets expected PCR7 and PCR11 values in the swtpm.
func measurePCRs() error {
	// Open the TPM.
	tpmDev, err := tpm2.OpenTPM("/dev/tpm0")
	if err != nil {
		return fmt.Errorf("can't open TPM: %s", err.Error())
	}
	defer tpmDev.Close()

	// Get a synthetic event log that should be used to populate the TPM's PCR values.
	rawLog, err := secureboot.SynthesizeTPMEventLog()
	if err != nil {
		return err
	}

	// Parse the event log.
	log, err := tcg.ParseEventLog(rawLog, tcg.ParseOpts{})
	if err != nil {
		return err
	}

	events := log.Events(register.HashSHA256)

	// Measure each event into the TPM.
	for _, event := range events {
		pcr := tpmutil.Handle(event.Index) //nolint:gosec

		err := tpm2.PCRExtend(tpmDev, pcr, tpm2.AlgSHA256, event.ReplayedDigest(), "")
		if err != nil {
			return err
		}
	}

	// Measure the "enter-initrd" userspace TPM event into PCR11.
	h := sha256.Sum256([]byte("enter-initrd"))

	err = tpm2.PCRExtend(tpmDev, tpmutil.Handle(11), tpm2.AlgSHA256, h[:], "")
	if err != nil {
		return err
	}

	return nil
}
