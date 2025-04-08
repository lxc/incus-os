package response

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// etagHash hashes the provided data and returns the sha256.
func etagHash(data any) (string, error) {
	etag := sha256.New()
	err := json.NewEncoder(etag).Encode(data)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(etag.Sum(nil)), nil
}
