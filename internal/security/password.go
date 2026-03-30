package security

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2id parameters - OWASP recommended as of 2024
	argonTime      = 3
	argonMemory    = 64 * 1024 // 64 MB
	argonThreads   = 4
	argonKeyLen    = 32
	argonSaltLen   = 16
	algorithmName  = "argon2id"
	minPasswordLen = 12
)

var hibpAPIURL = "https://api.pwnedpasswords.com/range/"

// HashPassword hashes a password using argon2id with OWASP-recommended parameters
// Returns the hash in the format: $argon2id$v=19$t=3,m=65536,p=4$<base64salt>$<base64hash>
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	// Generate random salt
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash password with argon2id
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Format: $argon2id$v=19$t=3,m=65536,p=4$<base64salt>$<base64hash>
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf("$argon2id$v=%d$t=%d,m=%d,p=%d$%s$%s",
		argon2.Version, argonTime, argonMemory, argonThreads, b64Salt, b64Hash)

	return encodedHash, nil
}

// VerifyPassword verifies a password against a hash using constant-time comparison
// Returns true if the password matches the hash
func VerifyPassword(password, encodedHash string) (bool, error) {
	if password == "" || encodedHash == "" {
		return false, nil
	}

	// Parse the hash format
	// Expected: $argon2id$v=19$t=3,m=65536,p=4$<base64salt>$<base64hash>
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != algorithmName {
		return false, fmt.Errorf("invalid hash format")
	}

	// Parse version
	if parts[2] != "v=19" {
		return false, fmt.Errorf("unsupported argon2 version: %s", parts[2])
	}

	// Parse parameters
	var params struct {
		time    uint32
		memory  uint32
		threads uint8
	}
	_, err := fmt.Sscanf(parts[3], "t=%d,m=%d,p=%d", &params.time, &params.memory, &params.threads)
	if err != nil {
		return false, fmt.Errorf("invalid parameters: %w", err)
	}

	// Decode salt and hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("invalid salt encoding: %w", err)
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("invalid hash encoding: %w", err)
	}

	// Hash the provided password with the same parameters
	computedHash := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, argonKeyLen)

	// Constant-time comparison to prevent timing attacks
	if len(computedHash) != len(decodedHash) {
		return false, nil
	}

	match := subtle.ConstantTimeCompare(computedHash, decodedHash) == 1
	return match, nil
}

// CheckPasswordBreach checks if a password has been exposed in known data breaches
// using the HIBP k-anonymity API (opt-in, configurable)
// Returns true if the password is compromised
func CheckPasswordBreach(password string) (bool, error) {
	if password == "" {
		return false, nil
	}

	// Compute SHA-1 hash of the password
	hash := sha1Hash(password)
	if len(hash) < 40 {
		return false, fmt.Errorf("failed to compute password hash")
	}

	// Send first 5 characters to HIBP
	prefix := hash[:5]
	suffix := hash[5:]

	url := fmt.Sprintf("%s%s", hibpAPIURL, prefix)
	resp, err := http.Get(url)
	if err != nil {
		// Don't fail the whole operation if HIBP is down
		// Log the error but return "not compromised" to allow login
		return false, fmt.Errorf("failed to check breach database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Same as above - don't fail on non-200
		return false, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, nil
	}

	// Check if the suffix is in the response
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), ":")
		if len(parts) == 2 && strings.EqualFold(parts[0], suffix) {
			// Password found in breach database
			return true, nil
		}
	}

	return false, nil
}

// ValidatePassword validates a password meets minimum requirements
func ValidatePassword(password string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}
	return nil
}

// sha1Hash computes the SHA-1 hash of a string (for HIBP k-anonymity check)
// This is NOT for password storage, only for breach checking
func sha1Hash(input string) string {
	h := sha1.Sum([]byte(input))
	return fmt.Sprintf("%x", h[:])
}

// ConstantTimeCompare is a helper for constant-time string comparison
// Used for comparing tokens and credentials without timing leakage
func ConstantTimeCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
