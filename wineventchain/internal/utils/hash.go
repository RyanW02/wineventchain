package utils

import "crypto/sha256"

func Sha256Sum(bytes []byte) []byte {
	digest := sha256.New()
	digest.Write(bytes)
	return digest.Sum(nil)
}
