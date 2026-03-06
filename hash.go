package main

import (
	"crypto/sha256"
	"encoding/hex"
)

func hashURL(u string) string {
	h := sha256.Sum256([]byte(u))
	// Reduces collision probability.
	return hex.EncodeToString(h[:16])
}
