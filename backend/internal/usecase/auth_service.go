package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

type AuthService struct {
	users     ports.UserRepository
	providers map[string]ports.AuthProvider
	jwtSecret []byte
	jwtTTL    time.Duration
}

func NewAuthService(users ports.UserRepository, providers []ports.AuthProvider, jwtSecret []byte, jwtTTL time.Duration) *AuthService {
	m := make(map[string]ports.AuthProvider, len(providers))
	for _, p := range providers {
		m[p.Name()] = p
	}
	return &AuthService{users: users, providers: m, jwtSecret: jwtSecret, jwtTTL: jwtTTL}
}

// AuthorizeURL returns the provider's redirect target plus the state value
// the caller must stash (e.g. in a short-lived cookie) to verify on
// callback.
func (s *AuthService) AuthorizeURL(providerName string) (url, state string, err error) {
	p, ok := s.providers[providerName]
	if !ok {
		return "", "", ErrUnknownProvider
	}
	state, err = newToken()
	if err != nil {
		return "", "", fmt.Errorf("usecase: generate state: %w", err)
	}
	return p.AuthorizeURL(state), state, nil
}

// HandleCallback exchanges the code, upserts the User (FindByEmail then
// Save if not found), and issues a signed session JWT.
func (s *AuthService) HandleCallback(ctx context.Context, providerName, code string) (token string, user *domain.User, err error) {
	p, ok := s.providers[providerName]
	if !ok {
		return "", nil, ErrUnknownProvider
	}

	info, err := p.ExchangeCode(ctx, code)
	if err != nil {
		return "", nil, fmt.Errorf("usecase: exchange code: %w", err)
	}

	user, err = s.users.FindByEmail(ctx, info.Email)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		user, err = domain.NewUser(info.Email, info.DisplayName, providerName, info.AvatarURL)
		if err != nil {
			return "", nil, err
		}
		if err := s.users.Save(ctx, user); err != nil {
			return "", nil, fmt.Errorf("usecase: save user: %w", err)
		}
	case err != nil:
		return "", nil, fmt.Errorf("usecase: lookup user: %w", err)
	default:
		if user.DisplayName != info.DisplayName || user.AvatarURL != info.AvatarURL {
			user.DisplayName = info.DisplayName
			user.AvatarURL = info.AvatarURL
			if err := s.users.Update(ctx, user); err != nil {
				return "", nil, fmt.Errorf("usecase: update user: %w", err)
			}
		}
	}

	token, err = s.issueJWT(user.ID)
	if err != nil {
		return "", nil, err
	}
	return token, user, nil
}

func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.users.FindByID(ctx, userID)
}

// VerifyToken validates a session JWT and extracts the user ID claim, for
// use by the HTTP-layer auth middleware.
func (s *AuthService) VerifyToken(token string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return s.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return uuid.Nil, fmt.Errorf("usecase: verify token: %w", err)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("usecase: invalid subject claim: %w", err)
	}
	return userID, nil
}

func (s *AuthService) issueJWT(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtTTL)),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("usecase: sign token: %w", err)
	}
	return signed, nil
}
