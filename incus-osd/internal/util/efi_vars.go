package util

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// ReadEFIVariable returns the current value, if any, of the specified EFI variable.
func ReadEFIVariable(variableName string) ([]byte, error) {
	// Determine which file to open.
	filename, err := EfiVariableToFilename(variableName)
	if err != nil {
		return nil, err
	}

	// Open the file.
	// #nosec G304
	file, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// If the EFI variable doesn't exist, return an empty buffer.
			return nil, nil
		}

		return nil, err
	}
	defer file.Close()

	// Get the contents of the EFI variable.
	s, err := file.Stat()
	if err != nil {
		return nil, err
	}

	buf := make([]byte, s.Size())

	numBytes, err := io.ReadFull(file, buf)
	if err != nil {
		return nil, err
	} else if int64(numBytes) != s.Size() {
		return nil, fmt.Errorf("only read %d bytes from %s", numBytes, filename)
	}

	// Trim the first four bytes; https://docs.kernel.org/filesystems/efivarfs.html
	return buf[4:], nil
}

// EfiVariableToFilename maps an EFI variable name to its file under /sys/.
func EfiVariableToFilename(variableName string) (string, error) {
	switch variableName {
	case "SecureBoot":
		return "/sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "SetupMode":
		return "/sys/firmware/efi/efivars/SetupMode-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "DeployedMode":
		return "/sys/firmware/efi/efivars/DeployedMode-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "AuditMode":
		return "/sys/firmware/efi/efivars/AuditMode-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "PK":
		return "/sys/firmware/efi/efivars/PK-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "KEK":
		return "/sys/firmware/efi/efivars/KEK-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "db":
		return "/sys/firmware/efi/efivars/db-d719b2cb-3d3a-4596-a3bc-dad00e67656f", nil
	case "dbx":
		return "/sys/firmware/efi/efivars/dbx-d719b2cb-3d3a-4596-a3bc-dad00e67656f", nil
	case "LoaderEntrySelected":
		return "/sys/firmware/efi/efivars/LoaderEntrySelected-4a67b082-0a4c-41cf-b6c7-440b29bb8c4f", nil
	case "IncusOSInstallComplete":
		return "/sys/firmware/efi/efivars/IncusOSInstallComplete-12f075e0-2d07-493d-811a-00920a72c04c", nil
	case "IncusOSTPMState":
		return "/sys/firmware/efi/efivars/IncusOSTPMState-12f075e0-2d07-493d-811a-00920a72c04c", nil
	default:
		return "", fmt.Errorf("unsupported EFI variable '%s'", variableName)
	}
}
