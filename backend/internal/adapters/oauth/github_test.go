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

func TestGitHubProvider_AuthorizeURL(t *testing.T) {
	p := NewGitHubProvider("client-id", "client-secret", "http://localhost:8008/api/auth/github/callback")
	url := p.AuthorizeURL("the-state")

	if !strings.HasPrefix(url, githubEndpoint.AuthURL) {
		t.Errorf("AuthorizeURL() = %q, want prefix %q", url, githubEndpoint.AuthURL)
	}
	if !strings.Contains(url, "state=the-state") {
		t.Errorf("AuthorizeURL() = %q, missing state param", url)
	}
	if !strings.Contains(url, "client_id=client-id") {
		t.Errorf("AuthorizeURL() = %q, missing client_id param", url)
	}
}

func TestGitHubProvider_ExchangeCode_PrivateEmailFallsBackToEmailsEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "fake-token",
			"token_type":   "bearer",
		})
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(githubUser{Login: "octocat", Name: "", Email: ""})
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]githubEmail{
			{Email: "secondary@example.com", Primary: false, Verified: true},
			{Email: "primary@example.com", Primary: true, Verified: true},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := &GitHubProvider{
		cfg: oauth2.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			Endpoint: oauth2.Endpoint{
				TokenURL: srv.URL + "/login/oauth/access_token",
			},
		},
		httpClient: srv.Client(),
	}
	// Redirect the hardcoded api.github.com URLs to our test server via a
	// transport that rewrites the host - simplest alternative: override the
	// userinfo URLs through a tiny client wrapper.
	p.httpClient = &http.Client{Transport: rewriteHostTransport{base: http.DefaultTransport, target: srv.URL}}

	info, err := p.ExchangeCode(context.Background(), "the-code")
	if err != nil {
		t.Fatalf("ExchangeCode() error: %v", err)
	}
	if info.Email != "primary@example.com" {
		t.Errorf("Email = %q, want %q", info.Email, "primary@example.com")
	}
	if info.DisplayName != "octocat" {
		t.Errorf("DisplayName = %q, want %q (falls back to login when name is empty)", info.DisplayName, "octocat")
	}
}

// rewriteHostTransport redirects any request to target's host, preserving
// path/query, so hardcoded provider API URLs can be exercised against a
// local httptest.Server in unit tests.
type rewriteHostTransport struct {
	base   http.RoundTripper
	target string
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	targetURL, err := req.URL.Parse(t.target + req.URL.Path)
	if err != nil {
		return nil, err
	}
	req2 := req.Clone(req.Context())
	req2.URL = targetURL
	req2.Host = ""
	return t.base.RoundTrip(req2)
}
