package http

import (
	"strings"
	"testing"
)

func TestContentDisposition(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // expected value of the filename="..." parameter
	}{
		{"simple", "vacation.jpg", "vacation.jpg"},
		{"no extension", "README", "README"},
		{"double extension keeps last", "archive.tar.gz", "archive.tar.gz"},
		{"empty falls back", "", "download"},
		{"path separators stripped", "../../etc/passwd", "passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contentDisposition(tt.input)
			if !strings.Contains(got, `filename="`+tt.want+`"`) {
				t.Errorf("contentDisposition(%q) = %q, want it to contain filename=%q", tt.input, got, tt.want)
			}
			if !strings.HasPrefix(got, "attachment;") {
				t.Errorf("contentDisposition(%q) = %q, want attachment; prefix", tt.input, got)
			}
		})
	}
}

func TestContentDisposition_RejectsHeaderInjection(t *testing.T) {
	malicious := "evil\r\nSet-Cookie: pwned=1\".jpg"

	got := contentDisposition(malicious)
	if strings.ContainsAny(got, "\r\n") {
		t.Errorf("contentDisposition() leaked a control character: %q", got)
	}
}
