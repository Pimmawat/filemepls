package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestGoogleProvider_AuthorizeURL(t *testing.T) {
	p := NewGoogleProvider("client-id", "client-secret", "http://localhost:8080/api/auth/google/callback")
	url := p.AuthorizeURL("the-state")

	if !strings.HasPrefix(url, googleEndpoint.AuthURL) {
		t.Errorf("AuthorizeURL() = %q, want prefix %q", url, googleEndpoint.AuthURL)
	}
	if !strings.Contains(url, "state=the-state") {
		t.Errorf("AuthorizeURL() = %q, missing state param", url)
	}
}

func TestGoogleProvider_ExchangeCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "fake-token",
			"token_type":   "bearer",
		})
	})
	mux.HandleFunc("/oauth2/v3/userinfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(googleUserInfo{
			Email:         "user@example.com",
			EmailVerified: true,
			Name:          "Test User",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := &GoogleProvider{
		cfg: oauth2.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			Endpoint: oauth2.Endpoint{
				TokenURL: srv.URL + "/token",
			},
		},
		httpClient: &http.Client{Transport: rewriteHostTransport{base: http.DefaultTransport, target: srv.URL}},
	}

	info, err := p.ExchangeCode(context.Background(), "the-code")
	if err != nil {
		t.Fatalf("ExchangeCode() error: %v", err)
	}
	if info.Email != "user@example.com" || info.DisplayName != "Test User" {
		t.Errorf("got %+v, want email=user@example.com displayName=Test User", info)
	}
}

func TestGoogleProvider_ExchangeCode_UnverifiedEmailRejected(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "fake-token"})
	})
	mux.HandleFunc("/oauth2/v3/userinfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(googleUserInfo{Email: "user@example.com", EmailVerified: false, Name: "Test User"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := &GoogleProvider{
		cfg: oauth2.Config{
			Endpoint: oauth2.Endpoint{TokenURL: srv.URL + "/token"},
		},
		httpClient: &http.Client{Transport: rewriteHostTransport{base: http.DefaultTransport, target: srv.URL}},
	}

	if _, err := p.ExchangeCode(context.Background(), "the-code"); err == nil {
		t.Fatal("expected an error for unverified email, got nil")
	}
}
