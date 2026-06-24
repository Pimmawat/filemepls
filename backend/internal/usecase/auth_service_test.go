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
	svc := NewAuthService(users, []ports.AuthProvider{provider}, []byte("test-secret"), time.Hour)
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
	svcA := NewAuthService(users, []ports.AuthProvider{provider}, []byte("secret-a"), time.Hour)
	svcB := NewAuthService(users, []ports.AuthProvider{provider}, []byte("secret-b"), time.Hour)

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
