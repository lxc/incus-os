package util

import (
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf16"
)

// UTF16ToString converts a byte array holding raw UTF-16 content to a string.
func UTF16ToString(buf []byte) (string, error) {
	if len(buf)%2 != 0 {
		return "", errors.New("UTF-16 buffer must contain an even number of bytes")
	}

	ubuf := make([]uint16, len(buf)/2)
	for i := 0; i < len(buf); i += 2 {
		ubuf[i/2] = binary.LittleEndian.Uint16(buf[i : i+2])
	}

	runes := utf16.Decode(ubuf)

	// Trim any trailing C-style NULL byte(s) from the string.
	return strings.TrimRight(string(runes), "\x00"), nil
}
