package organization

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes adds organization endpoints to the supplied route group.
func RegisterRoutes(r chi.Router, handler *Handler) {
	r.Get("/{organizationID}", func(w http.ResponseWriter, req *http.Request) {
		req.SetPathValue("organizationID", chi.URLParam(req, "organizationID"))
		handler.Get(w, req)
	})
	r.Post("/", handler.Create)
}
