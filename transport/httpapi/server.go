package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/siddharth-kulshrestha/taskforge/transport/httpapi/organization"
)

// Dependencies contains the feature handlers needed to register the API.
// Handlers use net/http and keep application services out of the router.
type Dependencies struct {
	Organizations *organization.Handler
	// RegisterAdditionalRoutes lets other feature packages register under /api/v1.
	RegisterAdditionalRoutes func(chi.Router)
	Logger                   *slog.Logger
}

// NewRouter builds a per-application router. It owns no global mutable state.
func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(requestIDHeader)
	r.Use(chimiddleware.Recoverer)
	if deps.Logger != nil {
		r.Use(requestLogger(deps.Logger))
	}

	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	r.Route("/api/v1", func(api chi.Router) {
		if deps.Organizations != nil {
			api.Route("/organizations", func(routes chi.Router) {
				organization.RegisterRoutes(routes, deps.Organizations)
			})
		}
		if deps.RegisterAdditionalRoutes != nil {
			deps.RegisterAdditionalRoutes(api)
		}
	})

	return r
}

func requestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := chimiddleware.GetReqID(r.Context()); id != "" {
			w.Header().Set(chimiddleware.RequestIDHeader, id)
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			next.ServeHTTP(w, r)
			logger.InfoContext(r.Context(), "http request",
				"method", r.Method,
				"path", r.URL.Path,
				"duration", time.Since(started),
				"request_id", chimiddleware.GetReqID(r.Context()),
			)
		})
	}
}
