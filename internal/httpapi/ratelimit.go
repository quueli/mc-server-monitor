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

// naive fixed-window limiter: count requests per ip in the current minute and
// reset when the window rolls over. bursty at the window edge, but simple.
type limiter struct {
	mu     sync.Mutex
	counts map[string]int
	window time.Time
	limit  int
}

func RateLimit(limit, _ int) func(http.Handler) http.Handler {
	l := &limiter{counts: make(map[string]int), window: time.Now(), limit: limit}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.limit))
			if !l.allow(clientIP(r)) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if time.Since(l.window) > time.Minute {
		l.counts = make(map[string]int)
		l.window = time.Now()
	}
	l.counts[ip]++
	return l.counts[ip] <= l.limit
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
