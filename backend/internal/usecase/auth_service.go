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

// minPasswordLength is enforced on registration only — an account created
// before this rule existed, or via an OAuth provider, is never re-validated
// against it.
const minPasswordLength = 8

type AuthService struct {
	users     ports.UserRepository
	providers map[string]ports.AuthProvider
	hasher    ports.PasswordHasher
	jwtSecret []byte
	jwtTTL    time.Duration
}

func NewAuthService(users ports.UserRepository, providers []ports.AuthProvider, hasher ports.PasswordHasher, jwtSecret []byte, jwtTTL time.Duration) *AuthService {
	m := make(map[string]ports.AuthProvider, len(providers))
	for _, p := range providers {
		m[p.Name()] = p
	}
	return &AuthService{users: users, providers: m, hasher: hasher, jwtSecret: jwtSecret, jwtTTL: jwtTTL}
}

// Register creates a new email+password account. The email must not
// already belong to any account, regardless of how that account
// authenticates (password or OAuth) — emails are globally unique.
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (token string, user *domain.User, err error) {
	if len(password) < minPasswordLength {
		return "", nil, domain.ErrWeakPassword
	}

	_, err = s.users.FindByEmail(ctx, email)
	switch {
	case err == nil:
		return "", nil, domain.ErrEmailAlreadyTaken
	case !errors.Is(err, domain.ErrNotFound):
		return "", nil, fmt.Errorf("usecase: lookup user: %w", err)
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return "", nil, fmt.Errorf("usecase: hash password: %w", err)
	}

	user, err = domain.NewLocalUser(email, displayName, hash)
	if err != nil {
		return "", nil, err
	}
	if err := s.users.Save(ctx, user); err != nil {
		return "", nil, fmt.Errorf("usecase: save user: %w", err)
	}

	token, err = s.issueJWT(user.ID)
	if err != nil {
		return "", nil, err
	}
	return token, user, nil
}

// Login verifies an email+password pair and issues a session JWT.
// Deliberately returns the same domain.ErrInvalidCredentials whether the
// email doesn't exist, belongs to an OAuth-only account (no password set),
// or the password is wrong — so a caller can't enumerate which emails are
// registered or how they authenticate.
func (s *AuthService) Login(ctx context.Context, email, password string) (token string, user *domain.User, err error) {
	user, err = s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", nil, domain.ErrInvalidCredentials
		}
		return "", nil, fmt.Errorf("usecase: lookup user: %w", err)
	}
	if user.PasswordHash == nil {
		return "", nil, domain.ErrInvalidCredentials
	}
	if err := s.hasher.Verify(*user.PasswordHash, password); err != nil {
		return "", nil, domain.ErrInvalidCredentials
	}

	token, err = s.issueJWT(user.ID)
	if err != nil {
		return "", nil, err
	}
	return token, user, nil
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
