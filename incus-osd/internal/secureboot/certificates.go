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

var (
	certX509SigGUID       = [16]byte{0xa1, 0x59, 0xc0, 0xa5, 0xe4, 0x94, 0xa7, 0x4a, 0x87, 0xb5, 0xab, 0x15, 0x5c, 0x2b, 0xf0, 0x72} // EFI_CERT_X509_GUID
	certHashSHA256SigGUID = [16]byte{0x92, 0xa4, 0xd2, 0x3b, 0xc0, 0x96, 0x79, 0x40, 0xb4, 0x20, 0xfc, 0xf9, 0x8e, 0xf1, 0x03, 0xed} // EFI_CERT_X509_SHA256_GUID
	certHashSHA384SigGUID = [16]byte{0x6e, 0x87, 0x76, 0x70, 0xc2, 0x80, 0xe6, 0x4e, 0xaa, 0xd2, 0x28, 0xb3, 0x49, 0xa6, 0x86, 0x5b} // EFI_CERT_X509_SHA384_GUID
	certHashSHA512SigGUID = [16]byte{0x63, 0xbf, 0x6d, 0x44, 0x02, 0x25, 0xda, 0x4c, 0xbc, 0xfa, 0x24, 0x65, 0xd2, 0xb0, 0xfe, 0x9d} // EFI_CERT_X509_SHA512_GUID
	hashSHA1SigGUID       = [16]byte{0x12, 0xa5, 0x6c, 0x82, 0x10, 0xcf, 0xc9, 0x4a, 0xb1, 0x87, 0xbe, 0x01, 0x49, 0x66, 0x31, 0xbd} // EFI_CERT_SHA1_GUID
	hashSHA224SigGUID     = [16]byte{0x33, 0x52, 0x6e, 0x0b, 0x5c, 0xa6, 0xc9, 0x44, 0x94, 0x07, 0xd9, 0xab, 0x83, 0xbf, 0xc8, 0xbd} // EFI_CERT_SHA224_GUID
	hashSHA256SigGUID     = [16]byte{0x26, 0x16, 0xc4, 0xc1, 0x4c, 0x50, 0x92, 0x40, 0xac, 0xa9, 0x41, 0xf9, 0x36, 0x93, 0x43, 0x28} // EFI_CERT_SHA256_GUID
	hashSHA384SigGUID     = [16]byte{0x07, 0x53, 0x3e, 0xff, 0xd0, 0x9f, 0xc9, 0x48, 0x85, 0xf1, 0x8a, 0xd5, 0x6c, 0x70, 0x1e, 0x01} // EFI_CERT_SHA384_GUID
	hashSHA512SigGUID     = [16]byte{0xae, 0x0f, 0x3e, 0x09, 0xc4, 0xa6, 0x50, 0x4f, 0x9f, 0x1b, 0xd4, 0x1e, 0x2b, 0x89, 0xc1, 0x9a} // EFI_CERT_SHA512_GUID
)

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
		case certHashSHA256SigGUID, certHashSHA384SigGUID, certHashSHA512SigGUID, // Hashes of a x509 certificate's To-Be-Signed contents
			hashSHA1SigGUID, hashSHA224SigGUID, hashSHA256SigGUID, hashSHA384SigGUID, hashSHA512SigGUID: // Hashes of an EFI binary
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

				// For certificate fingerprints, SignatureData consists of the hash of the certificate's
				// "To-Be-Signed", followed by an EFI_TIME (16 bytes) that indicates the time of revocation.
				// These entries only make sense in the dbx EFI variable. IncusOS doesn't make use of
				// hashes of dbx certificates, so nothing is done with the data we've just read.

				// For EFI binary hashes, SignatureData consists of the hash of an EFI binary that should
				// be whitelisted if in the db EFI variable, and blacklisted if in the dbx variable.
				// IncusOS doesn't make use of EFI binary hashes, so nothing is done with the data we've
				// just read.

				sigOffset += signatures.Header.SignatureSize
			}
		default:
			err = fmt.Errorf("unhandled signature EFI_GUID type '%x-%x-%x-%x-%x'", signatureType[:4], signatureType[4:6], signatureType[6:8], signatureType[8:10], signatureType[10:])
		}

		if err != nil {
			return nil, err
		}
	}

	return certificates, nil
}
