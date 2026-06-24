package http

import (
	"strings"
	"testing"
	"time"
)

func TestContentDisposition(t *testing.T) {
	createdAt := time.Date(2026, 6, 24, 15, 30, 12, 0, time.UTC)

	tests := []struct {
		name     string
		input    string
		wantPart string // substring expected inside the quoted filename
		wantExt  string // expected suffix of the whole header (the extension)
	}{
		{"simple", "vacation.jpg", "vacation_20260624153012", ".jpg"},
		{"no extension", "README", "README_20260624153012", ""},
		{"double extension keeps last", "archive.tar.gz", "archive.tar_20260624153012", ".gz"},
		{"empty falls back", "", "download_20260624153012", ""},
		{"path separators stripped", "../../etc/passwd", "passwd_20260624153012", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contentDisposition(tt.input, createdAt)
			if !strings.Contains(got, `filename="`+tt.wantPart+tt.wantExt+`"`) {
				t.Errorf("contentDisposition(%q) = %q, want it to contain filename=%q", tt.input, got, tt.wantPart+tt.wantExt)
			}
			if !strings.HasPrefix(got, "attachment;") {
				t.Errorf("contentDisposition(%q) = %q, want attachment; prefix", tt.input, got)
			}
		})
	}
}

func TestContentDisposition_RejectsHeaderInjection(t *testing.T) {
	createdAt := time.Now()
	malicious := "evil\r\nSet-Cookie: pwned=1\".jpg"

	got := contentDisposition(malicious, createdAt)
	if strings.ContainsAny(got, "\r\n") {
		t.Errorf("contentDisposition() leaked a control character: %q", got)
	}
}
