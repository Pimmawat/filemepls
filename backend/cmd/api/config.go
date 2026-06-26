package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	HTTPAddr           string
	FrontendBaseURL    string
	DefaultLocale      string
	CORSAllowedOrigins []string
	// CookieDomain sets the session cookie's Domain attribute. Leave empty
	// for a single-host deployment (cookie defaults to host-only). Set to
	// a shared parent domain (e.g. ".example.com") when the frontend and
	// backend live on different subdomains, so the browser-set cookie is
	// sent to both instead of being scoped to whichever one issued it.
	CookieDomain string

	DatabaseURL string

	JWTSecret string
	JWTTTL    time.Duration

	StorageRoot   string
	MaxUploadSize int64
	AllowedMimes  []string

	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURI  string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string
}

func loadConfig() config {
	loadDotEnv(".env")

	return config{
		HTTPAddr:           envOr("HTTP_ADDR", ":8008"),
		FrontendBaseURL:    envOr("FRONTEND_BASE_URL", "http://localhost:3003"),
		DefaultLocale:      envOr("DEFAULT_LOCALE", "th"),
		CORSAllowedOrigins: envCSVOr("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3003"}),
		CookieDomain:       os.Getenv("COOKIE_DOMAIN"),

		DatabaseURL: requireEnv("DATABASE_URL"),

		JWTSecret: requireEnv("JWT_SECRET"),
		JWTTTL:    envDurationOr("JWT_TTL", 24*time.Hour),

		StorageRoot:   envOr("STORAGE_ROOT", "./data/storage"),
		MaxUploadSize: envInt64Or("MAX_UPLOAD_SIZE", 100<<20), // 100MB
		AllowedMimes: envCSVOr("ALLOWED_MIMES", []string{
			"image/png", "image/jpeg", "image/gif", "image/webp",
			"application/pdf", "text/plain", "application/zip",
			"video/mp4", "audio/mpeg",
		}),

		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		GitHubRedirectURI:  envOr("GITHUB_REDIRECT_URI", "http://localhost:8008/api/auth/github/callback"),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:  envOr("GOOGLE_REDIRECT_URI", "http://localhost:8008/api/auth/google/callback"),
	}
}

// loadDotEnv reads KEY=VALUE pairs from path into the process environment,
// skipping blank lines and lines starting with #. A real environment
// variable already set always wins over the file (so e.g. an NSSM service's
// AppEnvironmentExtra, or a Docker env_file, can still override it) — this
// makes the file purely a fallback for local/manual runs. A missing file is
// not an error, since in some setups (Docker, NSSM-with-AppEnvironmentExtra)
// the environment is already fully populated without one.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required environment variable %s", key)
	}
	return v
}

func envCSVOr(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envInt64Or(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Fatalf("invalid value for %s: %v", key, err)
	}
	return n
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("invalid value for %s: %v", key, err)
	}
	return d
}
