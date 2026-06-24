package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewShareLinkForFile_InvalidVisibility(t *testing.T) {
	_, err := NewShareLinkForFile("tok", uuid.New(), Visibility("bogus"), nil, nil)
	if !errors.Is(err, ErrInvalidVisibility) {
		t.Fatalf("got err %v, want %v", err, ErrInvalidVisibility)
	}
}

func TestNewShareLinkForFile(t *testing.T) {
	fileID := uuid.New()
	s, err := NewShareLinkForFile("tok", fileID, VisibilityPublic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.TargetType != ShareTargetFile || s.FileID == nil || *s.FileID != fileID || s.FolderID != nil {
		t.Errorf("unexpected share link: %+v", s)
	}
}

func TestNewShareLinkForFolder(t *testing.T) {
	folderID := uuid.New()
	s, err := NewShareLinkForFolder("tok", folderID, VisibilityPublic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.TargetType != ShareTargetFolder || s.FolderID == nil || *s.FolderID != folderID || s.FileID != nil {
		t.Errorf("unexpected share link: %+v", s)
	}
}

func TestShareLink_IsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{"no expiry", nil, false},
		{"expired", &past, true},
		{"not yet expired", &future, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ShareLink{ExpiresAt: tt.expiresAt}
			if got := s.IsExpired(now); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShareLink_IsDownloadLimitReached(t *testing.T) {
	limit := 3
	tests := []struct {
		name  string
		max   *int
		count int
		want  bool
	}{
		{"unlimited", nil, 100, false},
		{"under limit", &limit, 2, false},
		{"at limit", &limit, 3, true},
		{"over limit", &limit, 4, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ShareLink{MaxDownloads: tt.max, DownloadCount: tt.count}
			if got := s.IsDownloadLimitReached(); got != tt.want {
				t.Errorf("IsDownloadLimitReached() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShareLink_RequiresPassword(t *testing.T) {
	hash := "hashed"
	if (&ShareLink{}).RequiresPassword() {
		t.Error("expected false when PasswordHash is nil")
	}
	if !(&ShareLink{PasswordHash: &hash}).RequiresPassword() {
		t.Error("expected true when PasswordHash is set")
	}
}

func TestShareLink_RecordDownload(t *testing.T) {
	limit := 2
	s := &ShareLink{MaxDownloads: &limit}

	if err := s.RecordDownload(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DownloadCount != 1 {
		t.Fatalf("count = %d, want 1", s.DownloadCount)
	}

	if err := s.RecordDownload(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DownloadCount != 2 {
		t.Fatalf("count = %d, want 2", s.DownloadCount)
	}

	if err := s.RecordDownload(); !errors.Is(err, ErrDownloadLimitHit) {
		t.Fatalf("got err %v, want %v", err, ErrDownloadLimitHit)
	}
	if s.DownloadCount != 2 {
		t.Fatalf("count should not increment past the limit, got %d", s.DownloadCount)
	}
}

func TestMatchesToken(t *testing.T) {
	if !MatchesToken("secret-token", "secret-token") {
		t.Error("expected matching tokens to match")
	}
	if MatchesToken("secret-token", "wrong-token") {
		t.Error("expected mismatched tokens to not match")
	}
	if MatchesToken("secret-token", "") {
		t.Error("expected empty candidate to not match")
	}
}
