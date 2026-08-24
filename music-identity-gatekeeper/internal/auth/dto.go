package auth

import "github.com/google/uuid"

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RegisterResponse struct {
	ID      uuid.UUID `json:"id"`
	Email   string    `json:"email"`
	Tier    string    `json:"tier"`
	Message string    `json:"message"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}
