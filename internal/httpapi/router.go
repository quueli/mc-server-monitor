package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

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

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/servers", a.listServers)
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
