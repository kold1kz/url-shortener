package auth

import (
	"strings"
	"testing"
)

func TestCookieName(t *testing.T) {
	if CookieName() != "auth" {
		t.Fatalf("expected auth, got %q", CookieName())
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	t.Setenv("TEST_AUTH_ENV", "custom-value")

	got := getEnvOrDefault("TEST_AUTH_ENV", "default-value")
	if got != "custom-value" {
		t.Fatalf("expected custom-value, got %q", got)
	}

	got = getEnvOrDefault("TEST_AUTH_ENV_MISSING", "default-value")
	if got != "default-value" {
		t.Fatalf("expected default-value, got %q", got)
	}
}

func TestGenerateUserID(t *testing.T) {
	id, err := generateUserID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}
	if len(id) != 32 {
		t.Fatalf("expected hex length 32, got %d", len(id))
	}
}

func TestSignUserID(t *testing.T) {
	token, err := signUserID("123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(token, "123:") {
		t.Fatalf("expected token to start with 123:, got %q", token)
	}
}

func TestParseToken_Success(t *testing.T) {
	token, err := signUserID("42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userID, err := parseToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "42" {
		t.Fatalf("expected 42, got %q", userID)
	}
}

func TestParseToken_EmptyToken(t *testing.T) {
	_, err := parseToken("")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "empty token" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseToken_BadFormat(t *testing.T) {
	_, err := parseToken("invalid-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "bad token format" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseToken_EmptyUserID(t *testing.T) {
	_, err := parseToken(":abcdef")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "empty user id" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseToken_InvalidHexSignature(t *testing.T) {
	_, err := parseToken("123:not-hex")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseToken_InvalidSignature(t *testing.T) {
	token, err := signUserID("42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	badToken := token[:len(token)-1] + "0"

	_, err = parseToken(badToken)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "invalid signature" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseToken_QueryEscaped(t *testing.T) {
	token, err := signUserID("42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	escaped := strings.ReplaceAll(token, ":", "%3A")

	userID, err := parseToken(escaped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "42" {
		t.Fatalf("expected 42, got %q", userID)
	}
}

func TestGetOrCreateUserIDFromCookie_ValidExistingCookie(t *testing.T) {
	token, err := signUserID("77")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userID, newToken, err := GetOrCreateUserIDFromCookie(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "77" {
		t.Fatalf("expected 77, got %q", userID)
	}
	if newToken != "" {
		t.Fatalf("expected empty new token, got %q", newToken)
	}
}

func TestGetOrCreateUserIDFromCookie_InvalidExistingCookieCreatesNew(t *testing.T) {
	userID, newToken, err := GetOrCreateUserIDFromCookie("bad-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID == "" {
		t.Fatal("expected generated userID")
	}
	if newToken == "" {
		t.Fatal("expected generated token")
	}

	parsedUserID, err := parseToken(newToken)
	if err != nil {
		t.Fatalf("generated token should be valid: %v", err)
	}
	if parsedUserID != userID {
		t.Fatalf("expected parsed userID %q, got %q", userID, parsedUserID)
	}
}

func TestGetOrCreateUserIDFromCookie_EmptyCookieCreatesNew(t *testing.T) {
	userID, newToken, err := GetOrCreateUserIDFromCookie("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID == "" {
		t.Fatal("expected generated userID")
	}
	if newToken == "" {
		t.Fatal("expected generated token")
	}
}

func TestGetUserIDFromCookieStrict(t *testing.T) {
	token, err := signUserID("15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userID, err := GetUserIDFromCookieStrict(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "15" {
		t.Fatalf("expected 15, got %q", userID)
	}
}

func TestGetUserIDFromCookieStrict_Empty(t *testing.T) {
	_, err := GetUserIDFromCookieStrict("")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "no cookie" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewTokenForUserID(t *testing.T) {
	token, err := NewTokenForUserID("101")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected token")
	}

	userID, err := parseToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "101" {
		t.Fatalf("expected 101, got %q", userID)
	}
}

func TestNewTokenForUserID_Empty(t *testing.T) {
	_, err := NewTokenForUserID("")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "empty user id" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseToken_ExportedWrapper(t *testing.T) {
	token, err := signUserID("555")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userID, err := ParseToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "555" {
		t.Fatalf("expected 555, got %q", userID)
	}
}
