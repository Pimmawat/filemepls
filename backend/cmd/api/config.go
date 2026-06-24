package main

import (
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
	return config{
		HTTPAddr:           envOr("HTTP_ADDR", ":8080"),
		FrontendBaseURL:    envOr("FRONTEND_BASE_URL", "http://localhost:3000"),
		DefaultLocale:      envOr("DEFAULT_LOCALE", "th"),
		CORSAllowedOrigins: envCSVOr("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),

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
		GitHubRedirectURI:  envOr("GITHUB_REDIRECT_URI", "http://localhost:8080/api/auth/github/callback"),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:  envOr("GOOGLE_REDIRECT_URI", "http://localhost:8080/api/auth/google/callback"),
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
