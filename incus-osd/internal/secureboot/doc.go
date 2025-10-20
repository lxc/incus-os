// Package secureboot implements logic related to handing secure boot
// key signing updates.
//
// NOTE -- It's assumed that PCR7 is the only one we care about in this code.
package secureboot

// TPMPCRMismatch holds the string returned by TPMStatus() if there's a PCR mismatch between the TPM and our computed value.
var TPMPCRMismatch = "pending PCR7 update"
