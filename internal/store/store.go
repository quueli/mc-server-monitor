package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a status update targets an unknown server.
var ErrNotFound = errors.New("store: server not found")

// Server is a monitored endpoint plus its most recent status snapshot.
type Server struct {
	ID         int64     `json:"id"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	Online     bool      `json:"online"`
	Players    int       `json:"players"`
	MaxPlayers int       `json:"max_players"`
	Version    string    `json:"version"`
	MOTD       string    `json:"motd"`
	LastCheck  time.Time `json:"last_check"`
}

// Status is a single snapshot written on every check.
type Status struct {
	Online     bool
	Players    int
	MaxPlayers int
	Version    string
	MOTD       string
	LatencyMS  int64
}

// Sample is one stored observation for the time series.
type Sample struct {
	ServerID   int64
	Online     bool
	Players    int
	MaxPlayers int
	LatencyMS  int64
	At         time.Time
}

type Store interface {
	UpsertServer(ctx context.Context, host string, port int) (int64, error)
	ListServers(ctx context.Context) ([]Server, error)
	UpdateStatus(ctx context.Context, serverID int64, st Status) error
	SaveSample(ctx context.Context, serverID int64, st Status) error
	RecentSamples(ctx context.Context, serverID int64, since time.Time) ([]Sample, error)
	PruneSamples(ctx context.Context, before time.Time) (int64, error)
}
