package http

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter is a small fixed-window, per-client-IP limiter for endpoints
// that must not be brute-forceable (login, register). It is intentionally
// dependency-free and in-process; a multi-instance deployment behind a load
// balancer should move this to a shared store (e.g. Redis) instead.
//
// IMPORTANT: ClientIP() is only trustworthy when gin's trusted-proxy list is
// configured correctly, otherwise a client can spoof X-Forwarded-For. Set
// engine.SetTrustedProxies(...) for the real proxy in front of this service.
type RateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
}

// NewRateLimiter allows at most max requests per client IP within window. It
// starts a background janitor that drops stale IP buckets so the map can't
// grow without bound.
func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{hits: make(map[string][]time.Time), max: max, window: window}
	go rl.janitor()
	return rl
}

func (rl *RateLimiter) allow(ip string, now time.Time) bool {
	cut := now.Add(-rl.window)
	rl.mu.Lock()
	defer rl.mu.Unlock()

	kept := rl.hits[ip][:0]
	for _, t := range rl.hits[ip] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.max {
		rl.hits[ip] = kept
		return false
	}
	rl.hits[ip] = append(kept, now)
	return true
}

func (rl *RateLimiter) janitor() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for now := range ticker.C {
		cut := now.Add(-rl.window)
		rl.mu.Lock()
		for ip, ts := range rl.hits {
			kept := ts[:0]
			for _, t := range ts {
				if t.After(cut) {
					kept = append(kept, t)
				}
			}
			if len(kept) == 0 {
				delete(rl.hits, ip)
			} else {
				rl.hits[ip] = kept
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware aborts with 429 when the caller's IP exceeds the configured rate.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP(), time.Now()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, try again later"})
			return
		}
		c.Next()
	}
}
