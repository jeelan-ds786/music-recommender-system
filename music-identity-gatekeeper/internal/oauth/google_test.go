package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestGoogleProviderExchangeUsesMockedProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"provider-token","token_type":"Bearer","expires_in":3600}`))
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer provider-token" {
				t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sub":"google-sub","email":"listener@example.com","email_verified":true,"name":"Listener"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := NewGoogleProvider("client-id", "client-secret", "http://localhost/callback")
	provider.config.Endpoint = oauth2.Endpoint{
		AuthURL:  server.URL + "/authorize",
		TokenURL: server.URL + "/token",
	}
	provider.userInfoURL = server.URL + "/userinfo"
	identity, err := provider.Exchange(context.Background(), "authorization-code")
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if identity.Subject != "google-sub" || identity.Email != "listener@example.com" || !identity.EmailVerified {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestGoogleAuthorizationURLContainsStateAndCallback(t *testing.T) {
	provider := NewGoogleProvider("client-id", "client-secret", "http://localhost:8080/auth/google/callback")
	url := provider.AuthorizationURL("opaque-state")
	for _, value := range []string{"state=opaque-state", "client_id=client-id", "redirect_uri="} {
		if !strings.Contains(url, value) {
			t.Fatalf("authorization URL %q missing %q", url, value)
		}
	}
}
