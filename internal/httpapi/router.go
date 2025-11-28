package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/quueli/mc-server-monitor/internal/mcping"
	"github.com/quueli/mc-server-monitor/internal/monitor"
	"github.com/quueli/mc-server-monitor/internal/store"
)

type API struct {
	store  store.Store
	poller *monitor.Poller
}

func NewAPI(st store.Store, p *monitor.Poller) *API {
	return &API{store: st, poller: p}
}

// Handler builds the router and the middleware chain. The two api routes carry
// their own per-IP limits; the status check is heavier so it gets a tighter one.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.Handle("GET /api/servers", RateLimit(60, 60)(http.HandlerFunc(a.listServers)))
	mux.Handle("GET /api/ping", RateLimit(10, 10)(http.HandlerFunc(a.ping)))
	return logRequests(mux)
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) listServers(w http.ResponseWriter, r *http.Request) {
	servers, err := a.store.ListServers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody("failed to list servers"))
		return
	}
	writeJSON(w, http.StatusOK, servers)
}

func (a *API) ping(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		writeJSON(w, http.StatusBadRequest, errorBody("host is required"))
		return
	}

	port := mcping.DefaultPort
	if raw := r.URL.Query().Get("port"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 65535 {
			writeJSON(w, http.StatusBadRequest, errorBody("invalid port"))
			return
		}
		port = n
	}

	status, err := a.poller.PingNow(host, port)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorBody("server unreachable"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"host":        host,
		"port":        port,
		"online":      true,
		"players":     status.Players.Online,
		"max_players": status.Players.Max,
		"version":     status.Version.Name,
		"motd":        status.MOTD.Clean(),
		"latency_ms":  status.LatencyMS,
		"favicon":     status.Favicon != "",
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func errorBody(msg string) map[string]string {
	return map[string]string{"error": msg}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
	})
}
