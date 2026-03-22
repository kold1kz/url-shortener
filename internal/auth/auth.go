package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
)

const (
	cookieName       = "auth"
	ContextUserIDKey = "userID"
)

var secretKey = getEnvOrDefault("SECRETKEY", "debugkey")

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func CookieName() string {
	return cookieName
}

func generateUserID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func signUserID(userID string) (string, error) {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(userID))
	sig := mac.Sum(nil)
	return userID + ":" + hex.EncodeToString(sig), nil
}

func parseToken(token string) (string, error) {
	if token == "" {
		return "", errors.New("empty token")
	}
	if decoded, err := url.QueryUnescape(token); err == nil {
		token = decoded
	}

	parts := strings.Split(token, ":")
	if len(parts) != 2 {
		return "", errors.New("bad token format")
	}

	userID := parts[0]
	if userID == "" {
		return "", errors.New("empty user id")
	}

	sigBytes, err := hex.DecodeString(parts[1])
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(userID))
	expected := mac.Sum(nil)

	if !hmac.Equal(sigBytes, expected) {
		return "", errors.New("invalid signature")
	}

	return userID, nil
}

func GetOrCreateUserIDFromCookie(rawCookie string) (string, string, error) {
	if rawCookie != "" {
		if userID, err := parseToken(rawCookie); err == nil {
			return userID, "", nil
		}
	}

	userID, err := generateUserID()
	if err != nil {
		return "", "", err
	}
	token, err := signUserID(userID)
	if err != nil {
		return "", "", err
	}
	return userID, token, nil
}

func GetUserIDFromCookieStrict(rawCookie string) (string, error) {
	if rawCookie == "" {
		return "", errors.New("no cookie")
	}
	return parseToken(rawCookie)
}

func NewTokenForUserID(userID string) (string, error) {
	if userID == "" {
		return "", errors.New("empty user id")
	}
	return signUserID(userID)
}

func ParseToken(token string) (string, error) {
	return parseToken(token)
}
