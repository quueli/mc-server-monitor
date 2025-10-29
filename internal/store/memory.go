package store

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"
)

type MemStore struct {
	mu      sync.RWMutex
	nextID  int64
	servers map[int64]*Server
	byAddr  map[string]int64
	samples []Sample
}

func NewMemStore() *MemStore {
	return &MemStore{
		servers: make(map[int64]*Server),
		byAddr:  make(map[string]int64),
	}
}

func addrKey(host string, port int) string {
	return host + ":" + strconv.Itoa(port)
}

func (m *MemStore) UpsertServer(_ context.Context, host string, port int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := addrKey(host, port)
	if id, ok := m.byAddr[key]; ok {
		return id, nil
	}
	m.nextID++
	id := m.nextID
	m.servers[id] = &Server{ID: id, Host: host, Port: port}
	m.byAddr[key] = id
	return id, nil
}

func (m *MemStore) ListServers(_ context.Context) ([]Server, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Server, 0, len(m.servers))
	for _, s := range m.servers {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *MemStore) UpdateStatus(_ context.Context, serverID int64, st Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.servers[serverID]
	if !ok {
		return ErrNotFound
	}
	s.Online = st.Online
	s.Players = st.Players
	s.MaxPlayers = st.MaxPlayers
	return nil
}

func (m *MemStore) SaveSample(_ context.Context, serverID int64, st Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.samples = append(m.samples, Sample{
		ServerID:   serverID,
		Online:     st.Online,
		Players:    st.Players,
		MaxPlayers: st.MaxPlayers,
		LatencyMS:  st.LatencyMS,
		At:         time.Now(),
	})
	return nil
}

func (m *MemStore) RecentSamples(_ context.Context, serverID int64, since time.Time) ([]Sample, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []Sample
	for _, s := range m.samples {
		if s.ServerID == serverID && s.At.After(since) {
			out = append(out, s)
		}
	}
	return out, nil
}

// TODO: actually drop old samples
func (m *MemStore) PruneSamples(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
