package metrics

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	logger *slog.Logger
	addr   string
}

func NewServer(logger *slog.Logger, addr string) *Server {
	return &Server{
		logger: logger,
		addr:   addr,
	}
}

func (s *Server) Listen() error {
	s.logger.Info("Starting metrics server", "addr", s.addr)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return http.ListenAndServe(s.addr, mux)
}
