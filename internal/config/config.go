package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr     string
	PollInterval time.Duration
	Concurrency  int
	PingTimeout  time.Duration
	CacheTTL     time.Duration
	Retention    time.Duration
	Servers      []Endpoint
	DBDriver     string
	DBDSN        string
	LogLevel     string
}

type Endpoint struct {
	Host string
	Port int
}

func Load() Config {
	return Config{
		HTTPAddr:     env("HTTP_ADDR", ":8080"),
		PollInterval: envDuration("POLL_INTERVAL", 5*time.Minute),
		Concurrency:  envInt("POLL_CONCURRENCY", 10),
		PingTimeout:  envDuration("PING_TIMEOUT", 10*time.Second),
		CacheTTL:     envDuration("CACHE_TTL", 2*time.Minute),
		Retention:    envDuration("SAMPLE_RETENTION", 14*24*time.Hour),
		Servers:      parseServers(env("MONITOR_SERVERS", "play.hypixel.net:25565")),
		DBDriver:     env("DB_DRIVER", ""),
		DBDSN:        env("DB_DSN", ""),
		LogLevel:     env("LOG_LEVEL", "info"),
	}
}

func parseServers(raw string) []Endpoint {
	var out []Endpoint
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		host, port := part, 25565
		if h, p, ok := strings.Cut(part, ":"); ok {
			host = h
			if n, err := strconv.Atoi(p); err == nil {
				port = n
			}
		}
		out = append(out, Endpoint{Host: host, Port: port})
	}
	return out
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
