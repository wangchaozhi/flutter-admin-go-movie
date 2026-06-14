package admin

import "testing"

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "s3cret" {
		t.Fatal("password was stored in plaintext")
	}
	if !looksHashed(hash) {
		t.Fatalf("expected bcrypt hash, got %q", hash)
	}
	if !checkPassword(hash, "s3cret") {
		t.Fatal("checkPassword rejected the correct password")
	}
	if checkPassword(hash, "wrong") {
		t.Fatal("checkPassword accepted the wrong password")
	}
}

func TestCheckPasswordLegacyPlaintext(t *testing.T) {
	// Rows seeded before hashing still store plaintext; login must keep working.
	if !checkPassword("123456", "123456") {
		t.Fatal("legacy plaintext password should match")
	}
	if checkPassword("123456", "000000") {
		t.Fatal("legacy plaintext password should not match a wrong value")
	}
}
