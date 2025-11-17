package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quueli/mc-server-monitor/internal/mcping"
	"github.com/quueli/mc-server-monitor/internal/store"
)

type Options struct {
	Interval    time.Duration
	Concurrency int
	PingTimeout time.Duration
	CacheTTL    time.Duration
	Retention   time.Duration
}

type Poller struct {
	pinger      *mcping.Pinger
	store       store.Store
	cache       *resultCache
	interval    time.Duration
	concurrency int
	retention   time.Duration
	running     int32
}

func NewPoller(st store.Store, opts Options) *Poller {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 10
	}
	if opts.Interval <= 0 {
		opts.Interval = 5 * time.Minute
	}
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = 2 * time.Minute
	}
	return &Poller{
		pinger:      mcping.New(opts.PingTimeout),
		store:       st,
		cache:       newResultCache(opts.CacheTTL),
		interval:    opts.Interval,
		concurrency: opts.Concurrency,
		retention:   opts.Retention,
	}
}

// Run checks every server once, then again on each tick, until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		slog.Warn("poller already running")
		return
	}
	defer atomic.StoreInt32(&p.running, 0)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.CheckAll(ctx)
	for {
		select {
		case <-ctx.Done():
			slog.Info("poller stopped")
			return
		case <-ticker.C:
			p.CheckAll(ctx)
		}
	}
}

// CheckAll pings every stored server. A channel semaphore caps how many run at
// once so a long server list does not open thousands of sockets at the same time.
func (p *Poller) CheckAll(ctx context.Context) {
	servers, err := p.store.ListServers(ctx)
	if err != nil {
		slog.Error("list servers", "err", err)
		return
	}
	if len(servers) == 0 {
		return
	}

	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(s store.Server) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("recovered panic while checking server", "host", s.Host, "panic", r)
				}
			}()

			sem <- struct{}{}
			defer func() { <-sem }()
			p.check(ctx, s)
		}(s)
	}
	wg.Wait()

	p.cache.purgeExpired()
	if p.retention > 0 {
		if _, err := p.store.PruneSamples(ctx, time.Now().Add(-p.retention)); err != nil {
			slog.Error("prune samples", "err", err)
		}
	}
}

func (p *Poller) check(ctx context.Context, s store.Server) {
	key := fmt.Sprintf("%s:%d", s.Host, s.Port)

	status, cached := p.cache.get(key)
	if !cached {
		var err error
		status, err = p.pinger.Ping(s.Host, s.Port)
		if err != nil {
			slog.Warn("ping failed", "host", s.Host, "err", err)
			offline := store.Status{Online: false}
			_ = p.store.UpdateStatus(ctx, s.ID, offline)
			_ = p.store.SaveSample(ctx, s.ID, offline)
			return
		}
		p.cache.set(key, status)
	}

	st := store.Status{
		Online:     true,
		Players:    status.Players.Online,
		MaxPlayers: status.Players.Max,
		Version:    status.Version.Name,
		MOTD:       status.MOTD.Clean(),
		LatencyMS:  status.LatencyMS,
	}
	if err := p.store.UpdateStatus(ctx, s.ID, st); err != nil {
		slog.Error("update status", "host", s.Host, "err", err)
	}
	if err := p.store.SaveSample(ctx, s.ID, st); err != nil {
		slog.Error("save sample", "host", s.Host, "err", err)
	}
}

// PingNow returns a cache-aware status for an ad-hoc host:port, used by the
// on-demand endpoint.
func (p *Poller) PingNow(host string, port int) (*mcping.Status, error) {
	key := fmt.Sprintf("%s:%d", host, port)
	if s, ok := p.cache.get(key); ok {
		return s, nil
	}
	s, err := p.pinger.Ping(host, port)
	if err != nil {
		return nil, err
	}
	p.cache.set(key, s)
	return s, nil
}
