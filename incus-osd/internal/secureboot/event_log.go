package secureboot

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"debug/pe"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf16"

	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"

	"github.com/lxc/incus-os/incus-osd/internal/util"
)

type eventLogHeader struct {
	pcrIndex       uint32
	eventType      tcg.EventType
	digest         [20]byte
	eventSize      uint32
	signature      [16]byte
	platformClass  uint32
	versionMinor   uint8
	versionMajor   uint8
	errata         uint8
	uintnSize      uint8
	numAlgs        uint32
	digestSizes    eventLogDigestSize
	vendorInfoSize uint8
}

type eventLogDigestSize struct {
	algID      uint16
	digestSize uint16
}

type event struct {
	name   string
	header eventHeader
}

type eventHeader struct {
	pcrIndex  uint32
	eventType tcg.EventType
	digests   struct {
		count   uint32
		digests eventDigest
	}
	eventSize uint32
}

type eventDigest struct {
	hash   uint16
	digest [32]byte
}

type efiSignatureListHeader struct {
	SignatureType       [16]byte
	SignatureListSize   uint32
	SignatureHeaderSize uint32
	SignatureSize       uint32
}

// SynthesizeTPMEventLog creates a very simple TPM event log covering expected PCR7 and PCR11 values
// that would have been measured while booting with a physical TPM. Since this code runs in user space
// post-boot, it is vulnerable to tampering by a malicious actor. When running swtpm, we rely on this
// event log to provide some basic TPM state validation.
//
// This should only ever be called to support running with swtpm. There are hard-coded assumptions that
// SHA256 is the only hashing function in use.
func SynthesizeTPMEventLog() ([]byte, error) {
	// A list of events that the TPM should measure into the log.
	events := []event{
		{
			name: "SecureBoot",
			header: eventHeader{
				pcrIndex:  7,
				eventType: tcg.EFIVariableDriverConfig,
			},
		},
		{
			name: "PK",
			header: eventHeader{
				pcrIndex:  7,
				eventType: tcg.EFIVariableDriverConfig,
			},
		},
		{
			name: "KEK",
			header: eventHeader{
				pcrIndex:  7,
				eventType: tcg.EFIVariableDriverConfig,
			},
		},
		{
			name: "db",
			header: eventHeader{
				pcrIndex:  7,
				eventType: tcg.EFIVariableDriverConfig,
			},
		},
		{
			name: "dbx",
			header: eventHeader{
				pcrIndex:  7,
				eventType: tcg.EFIVariableDriverConfig,
			},
		},
		{
			header: eventHeader{
				pcrIndex:  7,
				eventType: tcg.Separator,
			},
		},
		{
			name: "db",
			header: eventHeader{
				pcrIndex:  7,
				eventType: tcg.EFIVariableAuthority,
			},
		},
		{
			name: ".linux",
			header: eventHeader{
				pcrIndex:  11,
				eventType: tcg.Ipl,
			},
		},
		{
			name: ".osrel",
			header: eventHeader{
				pcrIndex:  11,
				eventType: tcg.Ipl,
			},
		},
		{
			name: ".cmdline",
			header: eventHeader{
				pcrIndex:  11,
				eventType: tcg.Ipl,
			},
		},
		{
			name: ".initrd",
			header: eventHeader{
				pcrIndex:  11,
				eventType: tcg.Ipl,
			},
		},
		{
			name: ".ucode",
			header: eventHeader{
				pcrIndex:  11,
				eventType: tcg.Ipl,
			},
		},
		{
			name: ".uname",
			header: eventHeader{
				pcrIndex:  11,
				eventType: tcg.Ipl,
			},
		},
		{
			name: ".sbat",
			header: eventHeader{
				pcrIndex:  11,
				eventType: tcg.Ipl,
			},
		},
		{
			name: ".pcrpkey",
			header: eventHeader{
				pcrIndex:  11,
				eventType: tcg.Ipl,
			},
		},
	}

	ukiImage, err := getUKIImage()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	// Write the event log header.
	header := eventLogHeader{
		pcrIndex:      0,
		eventType:     tcg.NoAction,
		digest:        [20]byte{}, // No digest for this entry
		eventSize:     33,
		signature:     [16]byte{0x53, 0x70, 0x65, 0x63, 0x20, 0x49, 0x44, 0x20, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x30, 0x33, 0x00}, // "Spec ID Event03"
		platformClass: 0,
		versionMinor:  0,
		versionMajor:  2,
		errata:        0,
		uintnSize:     2, // uint is 64 bits in size
		numAlgs:       1, // Hard-code only use of SHA256
		digestSizes: eventLogDigestSize{
			algID:      uint16(register.HashSHA256),
			digestSize: 32,
		},
		vendorInfoSize: 0,
	}

	err = binary.Write(&buf, binary.LittleEndian, header)
	if err != nil {
		return nil, err
	}

	// Iterate through each event and add it to the log.
	for _, e := range events {
		var contents []byte

		var err error

		switch e.header.eventType { //nolint:exhaustive
		case tcg.EFIVariableDriverConfig, tcg.EFIVariableAuthority:
			contents, err = readEFIVariable(e.name)
			if err != nil {
				return nil, err
			}

			if e.header.eventType == tcg.EFIVariableAuthority {
				contents, err = getSigningCertBytes(contents)
				if err != nil {
					return nil, err
				}
			}

			s := tcg.UEFIVariableData{
				Header: tcg.UEFIVariableDataHeader{
					UnicodeNameLength:  uint64(len(e.name)),
					VariableDataLength: uint64(len(contents)),
				},
				UnicodeName:  utf16.Encode([]rune(e.name)),
				VariableData: contents,
			}

			// Setting the proper GUID is a bit verbose, since the efiGUID struct from the tcg package isn't public.
			if e.name == "db" || e.name == "dbx" {
				// EFI_IMAGE_SECURITY_DATABASE_GUID
				s.Header.VariableName.Data1 = 0xd719b2cb
				s.Header.VariableName.Data2 = 0x3d3a
				s.Header.VariableName.Data3 = 0x4596
				s.Header.VariableName.Data4 = [8]byte{0xa3, 0xbc, 0xda, 0xd0, 0x0e, 0x67, 0x65, 0x6f}
			} else {
				// EFI_GLOBAL_VARIABLE_GUID
				s.Header.VariableName.Data1 = 0x8be4df61
				s.Header.VariableName.Data2 = 0x93ca
				s.Header.VariableName.Data3 = 0x11d2
				s.Header.VariableName.Data4 = [8]byte{0xaa, 0x0d, 0x00, 0xe0, 0x98, 0x03, 0x2b, 0x8c}
			}

			contents, err = s.Encode()
			if err != nil {
				return nil, err
			}
		case tcg.Ipl:
			// Microcode updates are currently only applied on amd64 systems. For arm64, we shouldn't
			// create an event log entry for the .ucode PE section.
			if e.name == ".ucode" && runtime.GOARCH != "amd64" {
				continue
			}

			// First entry: the name of the section with a trailing NULL byte.
			contents = []byte(e.name + "\x00")

			err = writeLogEvent(&buf, &e, contents)
			if err != nil {
				return nil, err
			}

			// Second entry: the binary contents of the PE section.
			peFile, err := pe.Open(ukiImage)
			if err != nil {
				return nil, err
			}

			defer peFile.Close() //nolint:revive

			peSection := peFile.Section(e.name)
			if peSection == nil {
				return nil, errors.New("failed to read PE section '" + e.name + "'")
			}

			c, err := peSection.Data()
			if err != nil {
				return nil, err
			}

			contents = c[0:peSection.VirtualSize]
		case tcg.Separator:
			contents = []byte{0x00, 0x00, 0x00, 0x00}
		default:
			return nil, errors.New("unsupported event type " + e.header.eventType.String())
		}

		err = writeLogEvent(&buf, &e, contents)
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func writeLogEvent(buf *bytes.Buffer, e *event, contents []byte) error {
	e.header.digests.count = 1
	e.header.digests.digests = eventDigest{
		hash:   uint16(register.HashSHA256),
		digest: sha256.Sum256(contents),
	}

	e.header.eventSize = uint32(len(contents)) //nolint:gosec

	err := binary.Write(buf, binary.LittleEndian, e.header)
	if err != nil {
		return err
	}

	err = binary.Write(buf, binary.LittleEndian, contents)
	if err != nil {
		return err
	}

	return nil
}

// This function is largely based on tcg.parseEfiSignatureList(), but that function drops the
// signature owner's GUID which we need when creating the event log.
func getSigningCertBytes(contents []byte) ([]byte, error) {
	maxDataLen := uint32(1024 * 1024)
	certX509SigGUID := [16]byte{0xa1, 0x59, 0xc0, 0xa5, 0xe4, 0x94, 0xa7, 0x4a, 0x87, 0xb5, 0xab, 0x15, 0x5c, 0x2b, 0xf0, 0x72} // EFI_CERT_X509_GUID

	// Get the RSA public key used by the running kernel.
	fd, err := os.Open("/run/systemd/tpm2-pcr-public-key.pem")
	if err != nil {
		return nil, err
	}

	pubKeyBytes, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(pubKeyBytes)

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an rsa public key")
	}

	buf := bytes.NewReader(contents)
	header := efiSignatureListHeader{}

	for buf.Len() > 0 {
		err := binary.Read(buf, binary.LittleEndian, &header)
		if err != nil {
			return nil, err
		}

		if header.SignatureHeaderSize > maxDataLen {
			return nil, fmt.Errorf("signature header too large: %d > %d", header.SignatureHeaderSize, maxDataLen)
		}

		if header.SignatureListSize > maxDataLen {
			return nil, fmt.Errorf("signature list too large: %d > %d", header.SignatureListSize, maxDataLen)
		}

		if header.SignatureType != certX509SigGUID {
			return nil, errors.New("only certX509SigGUID signatures are supported")
		}

		// Iterate through each certificate in the signature list and find the one that was
		// used to sign the UKI.
		for sigOffset := uint32(0); sigOffset < header.SignatureListSize-28; {
			signatureOwner := make([]byte, 16)
			signatureData := make([]byte, header.SignatureSize-16)

			err := binary.Read(buf, binary.LittleEndian, &signatureOwner)
			if err != nil {
				return nil, err
			}

			err = binary.Read(buf, binary.LittleEndian, &signatureData)
			if err != nil {
				return nil, err
			}

			cert, err := x509.ParseCertificate(signatureData)
			if err != nil {
				return nil, err
			}

			sigOffset += header.SignatureSize

			publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
			if !ok {
				return nil, errors.New("unsupported public key algorithm " + cert.PublicKeyAlgorithm.String())
			}

			// If we found the right certificate, return the bytes for just this certificate and its owner GUID.
			if rsaPubKey.Equal(publicKey) {
				var b bytes.Buffer

				_, err = b.Write(signatureOwner)
				if err != nil {
					return nil, err
				}

				_, err = b.Write(signatureData)
				if err != nil {
					return nil, err
				}

				return b.Bytes(), nil
			}
		}
	}

	return nil, errors.New("no cert found")
}

// Determine what UKI was booted, so we can compute the proper PCR11 values.
func getUKIImage() (string, error) {
	// Use the EFI variable LoaderEntrySelected to determine what UKI was booted.
	rawUKIName, err := readEFIVariable("LoaderEntrySelected")
	if err != nil {
		return "", err
	}

	ukiName, err := util.UTF16ToString(rawUKIName)
	if err != nil {
		return "", err
	}

	// Extract the IncusOS version that was booted. During OS upgrades, the EFI image is actually
	// renamed, so pull out the 12-digit version which will be unique, then do a readdir to find
	// the UKI image we need to examine.
	versionRegex := regexp.MustCompile(`^.+_(\d{12}).+efi$`)

	versionGroup := versionRegex.FindStringSubmatch(ukiName)
	if len(versionGroup) != 2 {
		return "", errors.New("unable to determine version from EFI variable LoaderEntrySelected ('" + ukiName + "')")
	}

	ukis, err := os.ReadDir("/boot/EFI/Linux/")
	if err != nil {
		return "", err
	}

	for _, uki := range ukis {
		if strings.Contains(uki.Name(), versionGroup[1]) {
			return "/boot/EFI/Linux/" + uki.Name(), nil
		}
	}

	return "", errors.New("unable to find UKI image for version " + versionGroup[1])
}
