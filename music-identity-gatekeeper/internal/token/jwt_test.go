package token

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseAccessToken(t *testing.T) {
	const secret = "test-secret"
	service := NewJWTService(secret)
	validToken, err := service.GenerateAccessToken("user-123", "local")
	if err != nil {
		t.Fatalf("generate valid token: %v", err)
	}

	expiredToken := signTestToken(t, secret, jwt.SigningMethodHS256, time.Now().Add(-time.Minute))
	wrongAlgorithmToken := signTestToken(t, secret, jwt.SigningMethodHS384, time.Now().Add(time.Minute))
	tamperedParts := strings.Split(validToken, ".")
	if tamperedParts[2][0] == 'a' {
		tamperedParts[2] = "b" + tamperedParts[2][1:]
	} else {
		tamperedParts[2] = "a" + tamperedParts[2][1:]
	}
	tamperedToken := strings.Join(tamperedParts, ".")

	tests := []struct {
		name      string
		token     string
		wantError bool
	}{
		{name: "valid", token: validToken},
		{name: "expired", token: expiredToken, wantError: true},
		{name: "malformed", token: "not-a-jwt", wantError: true},
		{name: "wrong algorithm", token: wrongAlgorithmToken, wantError: true},
		{name: "tampered", token: tamperedToken, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims, err := service.ParseAccessToken(test.token)
			if test.wantError {
				if err == nil {
					t.Fatal("ParseAccessToken() succeeded, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseAccessToken() error = %v", err)
			}
			if claims.UserID != "user-123" || claims.Provider != "local" {
				t.Fatalf("claims = %#v", claims)
			}
		})
	}
}

func signTestToken(
	t *testing.T,
	secret string,
	method jwt.SigningMethod,
	expiresAt time.Time,
) string {
	t.Helper()
	claims := Claims{
		UserID:   "user-123",
		Provider: "local",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signedToken, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return signedToken
}
