package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"

	"filemepls/internal/ports"
)

var _ ports.AuthProvider = (*GitHubProvider)(nil)

var githubEndpoint = oauth2.Endpoint{
	AuthURL:  "https://github.com/login/oauth/authorize",
	TokenURL: "https://github.com/login/oauth/access_token",
}

type GitHubProvider struct {
	cfg        oauth2.Config
	httpClient *http.Client
}

func NewGitHubProvider(clientID, clientSecret, redirectURI string) *GitHubProvider {
	return &GitHubProvider{
		cfg: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURI,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     githubEndpoint,
		},
		httpClient: http.DefaultClient,
	}
}

func (p *GitHubProvider) Name() string { return "github" }

func (p *GitHubProvider) AuthorizeURL(state string) string {
	return p.cfg.AuthCodeURL(state)
}

type githubUser struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (p *GitHubProvider) ExchangeCode(ctx context.Context, code string) (ports.ProviderUserInfo, error) {
	token, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return ports.ProviderUserInfo{}, fmt.Errorf("oauth/github: exchange code: %w", err)
	}

	var user githubUser
	if err := p.getJSON(ctx, token.AccessToken, "https://api.github.com/user", &user); err != nil {
		return ports.ProviderUserInfo{}, fmt.Errorf("oauth/github: fetch user: %w", err)
	}

	email := user.Email
	if email == "" {
		var emails []githubEmail
		if err := p.getJSON(ctx, token.AccessToken, "https://api.github.com/user/emails", &emails); err != nil {
			return ports.ProviderUserInfo{}, fmt.Errorf("oauth/github: fetch emails: %w", err)
		}
		for _, e := range emails {
			if e.Primary && e.Verified {
				email = e.Email
				break
			}
		}
	}
	if email == "" {
		return ports.ProviderUserInfo{}, fmt.Errorf("oauth/github: no verified email available")
	}

	displayName := user.Name
	if displayName == "" {
		displayName = user.Login
	}

	return ports.ProviderUserInfo{Email: email, DisplayName: displayName, AvatarURL: user.AvatarURL}, nil
}

func (p *GitHubProvider) getJSON(ctx context.Context, accessToken, url string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
