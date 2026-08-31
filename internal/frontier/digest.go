package frontier

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

func DigestBytes(raw []byte) string {
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func ValidateDigest(value string) error {
	if len(value) != len("sha256:")+64 || value[:len("sha256:")] != "sha256:" {
		return errors.New("digest must use sha256:<64 lowercase hex> format")
	}
	if _, err := hex.DecodeString(value[len("sha256:"):]); err != nil {
		return fmt.Errorf("invalid digest %q: %w", value, err)
	}
	return nil
}
