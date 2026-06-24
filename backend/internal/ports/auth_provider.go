package ports

import "context"

// AuthProvider abstracts an OAuth SSO provider (GitHub, Google, ...).
type AuthProvider interface {
	Name() string // "github" | "google"
	// AuthorizeURL builds the provider's consent-screen URL, embedding state
	// for CSRF protection on callback.
	AuthorizeURL(state string) string
	ExchangeCode(ctx context.Context, code string) (ProviderUserInfo, error)
}

type ProviderUserInfo struct {
	Email       string
	DisplayName string
	AvatarURL   string
}
