package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type VerificationToken struct {
	Raw       string
	Hash      string
	ExpiresAt time.Time
}

func NewVerificationToken(ttl time.Duration, now time.Time) (*VerificationToken, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	raw := hex.EncodeToString(buf)
	hash := sha256.Sum256([]byte(raw))
	return &VerificationToken{
		Raw:       raw,
		Hash:      hex.EncodeToString(hash[:]),
		ExpiresAt: now.Add(ttl),
	}, nil
}

func HashVerificationToken(raw string) string {
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}
