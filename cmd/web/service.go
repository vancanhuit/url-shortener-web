package main

import (
	"crypto/sha256"
	"encoding/base64"
)

func GenerateAlias(url string, requestID string) string {
	hash := sha256.Sum256([]byte(url + requestID))
	return base64.URLEncoding.EncodeToString(hash[:])[:11]
}
