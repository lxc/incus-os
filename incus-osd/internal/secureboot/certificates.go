package secureboot

import (
	"bytes"
	"crypto/x509"
	"encoding/binary"
	"fmt"
)

type parsedSignatureList struct {
	ownerGUID [16]byte
	cert      *x509.Certificate
	err       error
}

var maxDataLen = uint32(1024 * 1024)

var certX509SigGUID = [16]byte{0xa1, 0x59, 0xc0, 0xa5, 0xe4, 0x94, 0xa7, 0x4a, 0x87, 0xb5, 0xab, 0x15, 0x5c, 0x2b, 0xf0, 0x72} // EFI_CERT_X509_GUID

// parseEfiSignatureList is largely copied from tcg.parseEfiSignatureList(). It is modified to
// return additional information needed for each certificate and not to fail if a certificate
// fails to parse.
func parseEfiSignatureList(b []byte) ([]parsedSignatureList, error) {
	if len(b) < 28 {
		// Being passed an empty signature list here appears to be valid
		return nil, nil
	}

	signatures := efiSignatureList{}
	buf := bytes.NewReader(b)
	certificates := []parsedSignatureList{}

	for buf.Len() > 0 {
		err := binary.Read(buf, binary.LittleEndian, &signatures.Header)
		if err != nil {
			return nil, err
		}

		if signatures.Header.SignatureHeaderSize > maxDataLen {
			return nil, fmt.Errorf("signature header too large: %d > %d", signatures.Header.SignatureHeaderSize, maxDataLen)
		}

		if signatures.Header.SignatureListSize > maxDataLen {
			return nil, fmt.Errorf("signature list too large: %d > %d", signatures.Header.SignatureListSize, maxDataLen)
		}

		signatureType := signatures.Header.SignatureType
		switch signatureType {
		case certX509SigGUID: // X509 certificate
			for sigOffset := uint32(0); sigOffset < signatures.Header.SignatureListSize-28; {
				signature := efiSignatureData{}
				signature.SignatureData = make([]byte, signatures.Header.SignatureSize-16)

				err := binary.Read(buf, binary.LittleEndian, &signature.SignatureOwner)
				if err != nil {
					return nil, err
				}

				err = binary.Read(buf, binary.LittleEndian, &signature.SignatureData)
				if err != nil {
					return nil, err
				}

				cert, err := x509.ParseCertificate(signature.SignatureData)
				certificates = append(certificates, parsedSignatureList{
					ownerGUID: signature.SignatureOwner,
					cert:      cert,
					err:       err,
				})

				sigOffset += signatures.Header.SignatureSize
			}
		default:
			err = fmt.Errorf("unhandled signature type %s", signatureType)
		}

		if err != nil {
			return nil, err
		}
	}

	return certificates, nil
}
