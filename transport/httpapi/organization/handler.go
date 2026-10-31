package organization

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/siddharth-kulshrestha/taskforge/domain/models"
)

// UseCases is the application boundary used by the HTTP handler.
type UseCases interface {
	GetOrganizationByID(ctx context.Context, id uuid.UUID) (*models.Organization, error)
	CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error)
}

// Handler translates HTTP requests to organization use cases.
type Handler struct {
	app UseCases
}

func NewHandler(app UseCases) *Handler { return &Handler{app: app} }

type createRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("organizationID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}
	org, err := h.app.GetOrganizationByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, org)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input createRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	org, err := models.NewOrganization(input.Name, input.Slug)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.app.CreateOrganization(r.Context(), org)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": strings.TrimSpace(message)})
}
