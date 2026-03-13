// Package main provides IncusOS utility commands that run in the initrd.
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"
	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"

	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
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
		case "seal-pcr15":
			err = sealPCR15()
		case "validate-pe-binaries":
			err = secureboot.ValidatePEBinaries()
			if err != nil && os.IsNotExist(err) {
				// If we fail to find an expected PE binary, it might be because we're booting an installer on
				// a system where IncusOS is already installed resulting in two /boot/ partitions being present.
				// Because we're running in the initrd we can't easily determine which partition belongs to the
				// install media (that contains the PE binaries we should care about, since we're booting from
				// it); we can, however, restart the boot.mount systemd unit. Since we're now relatively late
				// in the initrd startup, udev symlinks have had a chance to settle and restarting boot.mount
				// seems to consistently mount the /boot/ partition from the install media. This is a heuristic,
				// er, hack that seems to work pretty well.
				//
				// Doing this would be problematic if swtpm was running, since its state is stored under /boot/,
				// but we only run PE binary validation if we have a physical TPM whose event log we can trust.

				// Sleep for one second before remounting /boot/. This ensures udev symlinks can fully settle,
				// even on hardware that might validate PE binaries very quickly.
				time.Sleep(1 * time.Second)

				serviceErr := systemd.RestartUnit(context.Background(), "boot.mount")
				if serviceErr == nil {
					err = secureboot.ValidatePEBinaries()
				}
			}
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
		pcr := tpmutil.Handle(event.Index) // #nosec G115

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

// sealPCR15 is used when running swtpm to extend PCR15 with a static value so it is
// initialized while in the initrd. Normally, this is done automatically by systemd-cryptsetup
// after unlocking the root LUKS volume.
func sealPCR15() error {
	// Open the TPM.
	tpmDev, err := tpm2.OpenTPM("/dev/tpm0")
	if err != nil {
		return fmt.Errorf("can't open TPM: %s", err.Error())
	}
	defer tpmDev.Close()

	// Measure a static value into PCR15.
	h := sha256.Sum256([]byte("IncusOS"))

	err = tpm2.PCRExtend(tpmDev, tpmutil.Handle(15), tpm2.AlgSHA256, h[:], "")
	if err != nil {
		return err
	}

	return nil
}
