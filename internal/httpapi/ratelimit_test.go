package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestLimiterAllowsBurstThenBlocks(t *testing.T) {
	l := &limiter{visitors: make(map[string]*bucket), rate: 60, burst: 3}

	for i := 0; i < 3; i++ {
		if ok, _, _ := l.allow("1.2.3.4"); !ok {
			t.Fatalf("request %d within burst should pass", i+1)
		}
	}
	if ok, remaining, _ := l.allow("1.2.3.4"); ok || remaining != 0 {
		t.Fatalf("request past the burst should be blocked, got ok=%v remaining=%d", ok, remaining)
	}
}

func TestLimiterTracksPerIP(t *testing.T) {
	l := &limiter{visitors: make(map[string]*bucket), rate: 60, burst: 1}

	if ok, _, _ := l.allow("10.0.0.1"); !ok {
		t.Fatal("first ip should pass")
	}
	if ok, _, _ := l.allow("10.0.0.2"); !ok {
		t.Fatal("a different ip has its own bucket and should pass")
	}
	if ok, _, _ := l.allow("10.0.0.1"); ok {
		t.Fatal("first ip is out of tokens and should be blocked")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.7:54321"
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("RemoteAddr: got %q, want 203.0.113.7", got)
	}

	r.Header.Set("X-Forwarded-For", "198.51.100.4, 203.0.113.7")
	if got := clientIP(r); got != "198.51.100.4" {
		t.Errorf("X-Forwarded-For: got %q, want 198.51.100.4", got)
	}
}
