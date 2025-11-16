package monitor

import (
	"context"
	"log/slog"
	"time"

	"github.com/quueli/mc-server-monitor/internal/mcping"
	"github.com/quueli/mc-server-monitor/internal/store"
)

type Poller struct {
	pinger   *mcping.Pinger
	store    store.Store
	interval time.Duration
}

func NewPoller(st store.Store, interval, timeout time.Duration) *Poller {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &Poller{pinger: mcping.New(timeout), store: st, interval: interval}
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.CheckAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.CheckAll(ctx)
		}
	}
}

// CheckAll pings each server in turn. Simple, but one slow server holds up the
// whole pass.
func (p *Poller) CheckAll(ctx context.Context) {
	servers, err := p.store.ListServers(ctx)
	if err != nil {
		slog.Error("list servers", "err", err)
		return
	}

	for _, s := range servers {
		status, err := p.pinger.Ping(s.Host, s.Port)
		if err != nil {
			slog.Warn("ping failed", "host", s.Host, "err", err)
			_ = p.store.UpdateStatus(ctx, s.ID, store.Status{Online: false})
			continue
		}
		st := store.Status{
			Online:     true,
			Players:    status.Players.Online,
			MaxPlayers: status.Players.Max,
			Version:    status.Version.Name,
			MOTD:       status.MOTD.Clean(),
			LatencyMS:  status.LatencyMS,
		}
		_ = p.store.UpdateStatus(ctx, s.ID, st)
		_ = p.store.SaveSample(ctx, s.ID, st)
	}
}
