package auth

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cret-pw")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "s3cret-pw" {
		t.Fatal("hash must not equal plaintext")
	}
	if !CheckPassword(hash, "s3cret-pw") {
		t.Fatal("correct password should verify")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("wrong password must not verify")
	}
}

func TestSessionTokenUniqueAndHashed(t *testing.T) {
	a, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("tokens must be unique")
	}
	if HashToken(a) == a {
		t.Fatal("stored hash must differ from raw token")
	}
	if HashToken(a) != HashToken(a) {
		t.Fatal("hashing must be deterministic")
	}
}
