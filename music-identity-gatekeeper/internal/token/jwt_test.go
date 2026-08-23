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
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	expiredToken := signTestToken(t, secret, jwt.SigningMethodHS256, time.Now().Add(-time.Minute))
	wrongAlgorithmToken := signTestToken(t, secret, jwt.SigningMethodHS384, time.Now().Add(time.Minute))
	parts := strings.Split(validToken, ".")
	tamperedToken := parts[0] + "." + parts[1] + ".tampered"

	tests := []struct {
		name      string
		token     string
		wantValid bool
	}{
		{name: "valid", token: validToken, wantValid: true},
		{name: "expired", token: expiredToken},
		{name: "malformed", token: "not-a-jwt"},
		{name: "wrong algorithm", token: wrongAlgorithmToken},
		{name: "tampered", token: tamperedToken},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims, err := service.ParseAccessToken(test.token)
			if test.wantValid {
				if err != nil {
					t.Fatalf("ParseAccessToken() error = %v", err)
				}
				if claims.UserID != "user-123" || claims.Provider != "local" {
					t.Fatalf("claims = %#v", claims)
				}
				return
			}
			if err == nil {
				t.Fatal("ParseAccessToken() error = nil, want rejection")
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
	signed, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
