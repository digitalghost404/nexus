package embed

import (
	"crypto/sha256"
	"encoding/hex"
)

func ContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func TruncateContent(content string, maxChars int) (string, bool) {
	if len(content) <= maxChars {
		return content, false
	}
	return content[:maxChars], true
}
