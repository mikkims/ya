package auth

import (
	"strings"
	"testing"
)

func TestTokenRoundTrip(t *testing.T) {
	token, err := NewToken("user-42")
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("token is not a JWT: %q", token)
	}
	userID, ok := UserIDFromToken(token)
	if !ok || userID != "user-42" {
		t.Fatalf("UserIDFromToken() = %q, %v", userID, ok)
	}
}

func TestTokenRejectsInvalidSignature(t *testing.T) {
	token, err := NewToken("user-42")
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	signatureStart := strings.LastIndex(token, ".") + 1
	last := token[signatureStart]
	replacement := byte('a')
	if last == replacement {
		replacement = 'b'
	}
	token = token[:signatureStart] + string(replacement) + token[signatureStart+1:]
	if _, ok := UserIDFromToken(token); ok {
		t.Fatal("token with invalid signature was accepted")
	}
}
