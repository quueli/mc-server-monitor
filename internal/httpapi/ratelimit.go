package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type bucket struct {
	tokens     int
	lastRefill time.Time
}

type limiter struct {
	mu       sync.Mutex
	visitors map[string]*bucket
	rate     int // tokens added per minute
	burst    int // bucket capacity
}

// RateLimit returns middleware that gives each client IP a token bucket: up to
// burst requests immediately, then one more per (60/rate) seconds.
func RateLimit(rate, burst int) func(http.Handler) http.Handler {
	l := &limiter{
		visitors: make(map[string]*bucket),
		rate:     rate,
		burst:    burst,
	}
	go l.cleanupLoop()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			ok, remaining, reset := l.allow(ip)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.burst))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))

			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *limiter) allow(ip string) (bool, int, time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.visitors[ip]
	if !ok {
		l.visitors[ip] = &bucket{tokens: l.burst - 1, lastRefill: now}
		return true, l.burst - 1, now.Add(time.Minute)
	}

	if refill := int(now.Sub(b.lastRefill).Minutes() * float64(l.rate)); refill > 0 {
		b.tokens += refill
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.lastRefill = now
	}

	if b.tokens <= 0 {
		return false, 0, b.lastRefill.Add(time.Minute)
	}
	b.tokens--
	return true, b.tokens, now.Add(time.Minute)
}

func (l *limiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		for ip, b := range l.visitors {
			if time.Since(b.lastRefill) > 5*time.Minute {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i >= 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
