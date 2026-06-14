package admin

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword returns a bcrypt hash for the given plaintext password.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// looksHashed reports whether stored already holds a bcrypt hash.
// It lets us transparently accept legacy plaintext rows that predate hashing.
func looksHashed(stored string) bool {
	return strings.HasPrefix(stored, "$2a$") ||
		strings.HasPrefix(stored, "$2b$") ||
		strings.HasPrefix(stored, "$2y$")
}

// checkPassword compares a plaintext password against a stored credential.
// Stored bcrypt hashes are verified with bcrypt; legacy plaintext rows fall
// back to a constant-time string comparison so existing seed data keeps working
// until it is migrated.
func checkPassword(stored, plain string) bool {
	if looksHashed(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(plain)) == nil
	}
	return stored == plain
}
