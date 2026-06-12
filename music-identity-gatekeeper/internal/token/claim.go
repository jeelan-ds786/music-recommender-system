package token

import (
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID      string `json:"user_id"`
	Provider    string `json:"auth_provider"`

	jwt.RegisteredClaims
}