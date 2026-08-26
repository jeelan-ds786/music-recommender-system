package token

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestParseAccessToken(t *testing.T) {
	const secret = "test-secret"
	service := NewJWTService(secret)
	validToken, err := service.GenerateAccessToken("user-123", "premium")
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
			if claims.UserID != "user-123" || claims.Tier != "premium" {
				t.Fatalf("claims = %#v", claims)
			}
			if _, err := uuid.Parse(claims.ID); err != nil {
				t.Fatalf("jti = %q, want UUID: %v", claims.ID, err)
			}
		})
	}
}

func TestGenerateAccessTokenUsesUniqueJTI(t *testing.T) {
	service := NewJWTService("test-secret")
	firstToken, err := service.GenerateAccessToken("user-123", "free")
	if err != nil {
		t.Fatalf("GenerateAccessToken() first error = %v", err)
	}
	secondToken, err := service.GenerateAccessToken("user-123", "free")
	if err != nil {
		t.Fatalf("GenerateAccessToken() second error = %v", err)
	}

	firstClaims, err := service.ParseAccessToken(firstToken)
	if err != nil {
		t.Fatalf("ParseAccessToken() first error = %v", err)
	}
	secondClaims, err := service.ParseAccessToken(secondToken)
	if err != nil {
		t.Fatalf("ParseAccessToken() second error = %v", err)
	}
	if firstClaims.ID == secondClaims.ID {
		t.Fatalf("jti = %q for both tokens, want unique values", firstClaims.ID)
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
		UserID: "user-123",
		Tier:   "premium",
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
