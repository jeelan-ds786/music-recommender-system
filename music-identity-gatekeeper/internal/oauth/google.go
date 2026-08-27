package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const googleUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

type Provider interface {
	AuthorizationURL(state string) string
	Exchange(ctx context.Context, code string) (*Identity, error)
}

type GoogleProvider struct {
	config      oauth2.Config
	userInfoURL string
}

func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{
		config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{"openid", "email", "profile"},
		},
		userInfoURL: googleUserInfoURL,
	}
}

func (p *GoogleProvider) AuthorizationURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *GoogleProvider) Exchange(ctx context.Context, code string) (_ *Identity, err error) {
	oauthToken, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := p.config.Client(ctx, oauthToken).Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close user info response: %w", closeErr)
		}
	}()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch user info: status %d", response.StatusCode)
	}

	var identity Identity
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&identity); err != nil {
		return nil, fmt.Errorf("decode user info: %w", err)
	}
	return &identity, nil
}

func (i *Identity) UnmarshalJSON(data []byte) error {
	type googleIdentity struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	var value googleIdentity
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	i.Subject = value.Subject
	i.Email = value.Email
	i.EmailVerified = value.EmailVerified
	i.Name = value.Name
	i.Picture = value.Picture
	return nil
}
