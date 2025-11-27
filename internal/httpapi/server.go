package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Server wraps http.Server with the timeouts we want and a graceful shutdown.
type Server struct {
	http *http.Server
}

func NewServer(addr string, handler http.Handler) *Server {
	return &Server{
		http: &http.Server{
			Addr:           addr,
			Handler:        handler,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
	}
}

func (s *Server) Start() error {
	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
