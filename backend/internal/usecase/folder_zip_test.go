package usecase

import "testing"

func TestSanitizeZipName(t *testing.T) {
	cases := map[string]string{
		"report.pdf":         "report.pdf",
		"../../etc/passwd":   "passwd",
		`..\..\Windows\evil`: "evil",
		"a/b/c.txt":          "c.txt",
		"..":                 "unnamed",
		".":                  "unnamed",
		"":                   "unnamed",
		`..\..\..\`:          "unnamed",
		"ไฟล์.txt":           "ไฟล์.txt", // unicode name preserved
	}
	for in, want := range cases {
		if got := sanitizeZipName(in); got != want {
			t.Errorf("sanitizeZipName(%q) = %q, want %q", in, got, want)
		}
	}
}
