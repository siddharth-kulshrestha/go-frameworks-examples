package httpapi

import (
	"net/http"
	"time"
)

// ServerConfig contains HTTP connection and timeout settings, separate from
// route registration.
type ServerConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

// NewServer creates an HTTP server using the supplied handler and settings.
func NewServer(config ServerConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              config.Addr,
		Handler:           handler,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
	}
}
