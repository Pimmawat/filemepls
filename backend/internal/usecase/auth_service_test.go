package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

func newTestAuthService(provider *fakeAuthProvider) (*AuthService, *fakeUserRepository) {
	users := newFakeUserRepository()
	svc := NewAuthService(users, []ports.AuthProvider{provider}, fakePasswordHasher{}, []byte("test-secret"), time.Hour)
	return svc, users
}

func TestAuthService_AuthorizeURL(t *testing.T) {
	svc, _ := newTestAuthService(&fakeAuthProvider{name: "github"})

	url, state, err := svc.AuthorizeURL("github")
	if err != nil {
		t.Fatalf("AuthorizeURL() error: %v", err)
	}
	if state == "" {
		t.Error("expected a non-empty state")
	}
	if url == "" {
		t.Error("expected a non-empty authorize URL")
	}
}

func TestAuthService_AuthorizeURL_UnknownProvider(t *testing.T) {
	svc, _ := newTestAuthService(&fakeAuthProvider{name: "github"})

	if _, _, err := svc.AuthorizeURL("bogus"); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("got %v, want %v", err, ErrUnknownProvider)
	}
}

func TestAuthService_HandleCallback_CreatesNewUser(t *testing.T) {
	provider := &fakeAuthProvider{name: "github", info: ports.ProviderUserInfo{Email: "new@example.com", DisplayName: "New User"}}
	svc, users := newTestAuthService(provider)

	token, user, err := svc.HandleCallback(context.Background(), "github", "the-code")
	if err != nil {
		t.Fatalf("HandleCallback() error: %v", err)
	}
	if token == "" {
		t.Error("expected a non-empty JWT")
	}
	if user.Email != "new@example.com" || user.Provider != "github" {
		t.Errorf("unexpected user: %+v", user)
	}

	stored, err := users.FindByEmail(context.Background(), "new@example.com")
	if err != nil {
		t.Fatalf("expected user to be persisted, got error: %v", err)
	}
	if stored.ID != user.ID {
		t.Errorf("persisted user ID mismatch: %v vs %v", stored.ID, user.ID)
	}
}

func TestAuthService_HandleCallback_ReusesExistingUser(t *testing.T) {
	provider := &fakeAuthProvider{name: "github", info: ports.ProviderUserInfo{Email: "existing@example.com", DisplayName: "Existing User"}}
	svc, users := newTestAuthService(provider)

	existing, err := domain.NewUser("existing@example.com", "Existing User", "github", "")
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}
	if err := users.Save(context.Background(), existing); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	_, user, err := svc.HandleCallback(context.Background(), "github", "the-code")
	if err != nil {
		t.Fatalf("HandleCallback() error: %v", err)
	}
	if user.ID != existing.ID {
		t.Errorf("expected the existing user to be reused, got a different ID: %v vs %v", user.ID, existing.ID)
	}
}

func TestAuthService_HandleCallback_UnknownProvider(t *testing.T) {
	svc, _ := newTestAuthService(&fakeAuthProvider{name: "github"})
	if _, _, err := svc.HandleCallback(context.Background(), "bogus", "code"); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("got %v, want %v", err, ErrUnknownProvider)
	}
}

func TestAuthService_HandleCallback_ExchangeError(t *testing.T) {
	provider := &fakeAuthProvider{name: "github", exchErr: errors.New("provider unavailable")}
	svc, _ := newTestAuthService(provider)
	if _, _, err := svc.HandleCallback(context.Background(), "github", "code"); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestAuthService_VerifyToken_RoundTrip(t *testing.T) {
	provider := &fakeAuthProvider{name: "github", info: ports.ProviderUserInfo{Email: "a@example.com", DisplayName: "A"}}
	svc, _ := newTestAuthService(provider)

	token, user, err := svc.HandleCallback(context.Background(), "github", "code")
	if err != nil {
		t.Fatalf("HandleCallback() error: %v", err)
	}

	gotID, err := svc.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken() error: %v", err)
	}
	if gotID != user.ID {
		t.Errorf("got %v, want %v", gotID, user.ID)
	}
}

func TestAuthService_VerifyToken_RejectsGarbage(t *testing.T) {
	svc, _ := newTestAuthService(&fakeAuthProvider{name: "github"})
	if _, err := svc.VerifyToken("not-a-real-jwt"); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestAuthService_VerifyToken_RejectsWrongSecret(t *testing.T) {
	provider := &fakeAuthProvider{name: "github", info: ports.ProviderUserInfo{Email: "a@example.com", DisplayName: "A"}}
	users := newFakeUserRepository()
	svcA := NewAuthService(users, []ports.AuthProvider{provider}, fakePasswordHasher{}, []byte("secret-a"), time.Hour)
	svcB := NewAuthService(users, []ports.AuthProvider{provider}, fakePasswordHasher{}, []byte("secret-b"), time.Hour)

	token, _, err := svcA.HandleCallback(context.Background(), "github", "code")
	if err != nil {
		t.Fatalf("HandleCallback() error: %v", err)
	}
	if _, err := svcB.VerifyToken(token); err == nil {
		t.Fatal("expected an error verifying a token signed with a different secret")
	}
}

func TestAuthService_Me(t *testing.T) {
	provider := &fakeAuthProvider{name: "github", info: ports.ProviderUserInfo{Email: "a@example.com", DisplayName: "A"}}
	svc, _ := newTestAuthService(provider)

	_, user, err := svc.HandleCallback(context.Background(), "github", "code")
	if err != nil {
		t.Fatalf("HandleCallback() error: %v", err)
	}

	got, err := svc.Me(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Me() error: %v", err)
	}
	if got.Email != user.Email {
		t.Errorf("got %v, want %v", got.Email, user.Email)
	}
}

func TestAuthService_Register(t *testing.T) {
	svc, users := newTestAuthService(&fakeAuthProvider{name: "github"})
	ctx := context.Background()

	token, user, err := svc.Register(ctx, "new@example.com", "correct-password", "New User")
	if err != nil {
		t.Fatalf("Register() error: %v", err)
	}
	if token == "" {
		t.Error("expected a non-empty JWT")
	}
	if user.Email != "new@example.com" || user.Provider != domain.ProviderPassword {
		t.Errorf("unexpected user: %+v", user)
	}

	stored, err := users.FindByEmail(ctx, "new@example.com")
	if err != nil {
		t.Fatalf("expected user to be persisted, got error: %v", err)
	}
	if stored.PasswordHash == nil {
		t.Error("expected the stored user to have a password hash")
	}
}

func TestAuthService_Register_RejectsDuplicateEmail(t *testing.T) {
	svc, _ := newTestAuthService(&fakeAuthProvider{name: "github"})
	ctx := context.Background()

	if _, _, err := svc.Register(ctx, "dup@example.com", "correct-password", "First"); err != nil {
		t.Fatalf("first Register() error: %v", err)
	}
	if _, _, err := svc.Register(ctx, "dup@example.com", "another-password", "Second"); !errors.Is(err, domain.ErrEmailAlreadyTaken) {
		t.Errorf("got %v, want %v", err, domain.ErrEmailAlreadyTaken)
	}
}

func TestAuthService_Register_RejectsDuplicateEmailFromOAuthAccount(t *testing.T) {
	provider := &fakeAuthProvider{name: "github", info: ports.ProviderUserInfo{Email: "oauth@example.com", DisplayName: "OAuth User"}}
	svc, _ := newTestAuthService(provider)
	ctx := context.Background()

	if _, _, err := svc.HandleCallback(ctx, "github", "code"); err != nil {
		t.Fatalf("HandleCallback() error: %v", err)
	}
	if _, _, err := svc.Register(ctx, "oauth@example.com", "correct-password", "Someone Else"); !errors.Is(err, domain.ErrEmailAlreadyTaken) {
		t.Errorf("got %v, want %v", err, domain.ErrEmailAlreadyTaken)
	}
}

func TestAuthService_Register_RejectsWeakPassword(t *testing.T) {
	svc, _ := newTestAuthService(&fakeAuthProvider{name: "github"})
	if _, _, err := svc.Register(context.Background(), "weak@example.com", "short", "Weak"); !errors.Is(err, domain.ErrWeakPassword) {
		t.Errorf("got %v, want %v", err, domain.ErrWeakPassword)
	}
}

func TestAuthService_Login(t *testing.T) {
	svc, _ := newTestAuthService(&fakeAuthProvider{name: "github"})
	ctx := context.Background()

	if _, _, err := svc.Register(ctx, "login@example.com", "correct-password", "Login User"); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	token, user, err := svc.Login(ctx, "login@example.com", "correct-password")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if token == "" {
		t.Error("expected a non-empty JWT")
	}
	if user.Email != "login@example.com" {
		t.Errorf("got %v, want login@example.com", user.Email)
	}
}

func TestAuthService_Login_RejectsWrongPassword(t *testing.T) {
	svc, _ := newTestAuthService(&fakeAuthProvider{name: "github"})
	ctx := context.Background()

	if _, _, err := svc.Register(ctx, "wrongpw@example.com", "correct-password", "User"); err != nil {
		t.Fatalf("Register() error: %v", err)
	}
	if _, _, err := svc.Login(ctx, "wrongpw@example.com", "wrong-password"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("got %v, want %v", err, domain.ErrInvalidCredentials)
	}
}

func TestAuthService_Login_RejectsUnknownEmail(t *testing.T) {
	svc, _ := newTestAuthService(&fakeAuthProvider{name: "github"})
	if _, _, err := svc.Login(context.Background(), "nobody@example.com", "whatever"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("got %v, want %v", err, domain.ErrInvalidCredentials)
	}
}

// TestAuthService_Login_RejectsOAuthOnlyAccount covers an account created
// via OAuth (no password set) — a credential-stuffing attempt against its
// email must fail the same way as a wrong password, not leak that the
// account exists but has no password.
func TestAuthService_Login_RejectsOAuthOnlyAccount(t *testing.T) {
	provider := &fakeAuthProvider{name: "github", info: ports.ProviderUserInfo{Email: "oauthonly@example.com", DisplayName: "OAuth Only"}}
	svc, _ := newTestAuthService(provider)
	ctx := context.Background()

	if _, _, err := svc.HandleCallback(ctx, "github", "code"); err != nil {
		t.Fatalf("HandleCallback() error: %v", err)
	}
	if _, _, err := svc.Login(ctx, "oauthonly@example.com", "anything"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("got %v, want %v", err, domain.ErrInvalidCredentials)
	}
}
