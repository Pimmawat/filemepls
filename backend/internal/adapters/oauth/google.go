package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"

	"filemepls/internal/ports"
)

var _ ports.AuthProvider = (*GoogleProvider)(nil)

var googleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

type GoogleProvider struct {
	cfg        oauth2.Config
	httpClient *http.Client
}

func NewGoogleProvider(clientID, clientSecret, redirectURI string) *GoogleProvider {
	return &GoogleProvider{
		cfg: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURI,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     googleEndpoint,
		},
		httpClient: http.DefaultClient,
	}
}

func (p *GoogleProvider) Name() string { return "google" }

func (p *GoogleProvider) AuthorizeURL(state string) string {
	return p.cfg.AuthCodeURL(state)
}

type googleUserInfo struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (ports.ProviderUserInfo, error) {
	token, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return ports.ProviderUserInfo{}, fmt.Errorf("oauth/google: exchange code: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return ports.ProviderUserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return ports.ProviderUserInfo{}, fmt.Errorf("oauth/google: fetch userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ports.ProviderUserInfo{}, fmt.Errorf("oauth/google: unexpected status %d from userinfo", resp.StatusCode)
	}

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return ports.ProviderUserInfo{}, fmt.Errorf("oauth/google: decode userinfo: %w", err)
	}
	if info.Email == "" || !info.EmailVerified {
		return ports.ProviderUserInfo{}, fmt.Errorf("oauth/google: no verified email available")
	}

	return ports.ProviderUserInfo{Email: info.Email, DisplayName: info.Name}, nil
}
