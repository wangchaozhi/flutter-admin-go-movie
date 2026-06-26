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

// CheckPasswordHash reports whether plain matches the stored credential. Exported
// so other packages (e.g. mobile change-password) can verify the current
// password without duplicating the legacy-plaintext fallback logic.
func CheckPasswordHash(stored, plain string) bool {
	return checkPassword(stored, plain)
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
