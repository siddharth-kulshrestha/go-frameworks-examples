package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/siddharth-kulshrestha/taskforge/domain/models"
	"github.com/siddharth-kulshrestha/taskforge/transport/httpapi/organization"
)

type organizationApp struct {
	gotID  uuid.UUID
	gotOrg *models.Organization
	err    error
}

func (a *organizationApp) GetOrganizationByID(_ context.Context, id uuid.UUID) (*models.Organization, error) {
	a.gotID = id
	if a.err != nil {
		return nil, a.err
	}
	return &models.Organization{ID: id, Name: "Example", Slug: "example"}, nil
}

func (a *organizationApp) CreateOrganization(_ context.Context, org *models.Organization) (*models.Organization, error) {
	a.gotOrg = org
	return org, a.err
}

func TestRouterOrganizationRoutes(t *testing.T) {
	app := &organizationApp{}
	id := uuid.New()
	handler := NewRouter(Dependencies{Organizations: organization.NewHandler(app)})

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+id.String()+"?view=full", nil))
	if get.Code != http.StatusOK || app.gotID != id {
		t.Fatalf("GET status/id = %d/%s, want %d/%s", get.Code, app.gotID, http.StatusOK, id)
	}
	var response models.Organization
	if err := json.Unmarshal(get.Body.Bytes(), &response); err != nil || response.ID != id {
		t.Fatalf("GET response = %q, err %v", get.Body.String(), err)
	}

	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/api/v1/organizations", strings.NewReader(`{"name":"Example","slug":"example"}`)))
	if post.Code != http.StatusCreated || app.gotOrg == nil || app.gotOrg.Slug != "example" {
		t.Fatalf("POST status/org = %d/%+v", post.Code, app.gotOrg)
	}

	badPath := httptest.NewRecorder()
	handler.ServeHTTP(badPath, httptest.NewRequest(http.MethodGet, "/api/v1/organizations/not-a-uuid", nil))
	if badPath.Code != http.StatusBadRequest {
		t.Fatalf("invalid ID status = %d, want %d", badPath.Code, http.StatusBadRequest)
	}
}

func TestRouterNestedRoutesAndMiddlewareContext(t *testing.T) {
	var requestID string
	var teamID, memberID, include string
	handler := NewRouter(Dependencies{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		RegisterAdditionalRoutes: func(api chi.Router) {
			api.Route("/teams", func(teams chi.Router) {
				teams.Route("/{teamID}/members", func(members chi.Router) {
					members.Get("/{memberID}", func(w http.ResponseWriter, r *http.Request) {
						requestID = chimiddleware.GetReqID(r.Context())
						teamID = chi.URLParam(r, "teamID")
						memberID = chi.URLParam(r, "memberID")
						include = r.URL.Query().Get("include")
						w.WriteHeader(http.StatusNoContent)
					})
				})
			})
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/teams/team-7/members/member-9?include=profile", nil)
	req.Header.Set("X-Request-ID", "test-request")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("nested route status = %d", res.Code)
	}
	if requestID != "test-request" || res.Header().Get("X-Request-ID") != "test-request" {
		t.Fatalf("request ID context/header = %q/%q", requestID, res.Header().Get("X-Request-ID"))
	}
	if teamID != "team-7" || memberID != "member-9" || include != "profile" {
		t.Fatalf("nested params/query = %q/%q/%q", teamID, memberID, include)
	}
}

func TestRouterNotFoundAndMethodNotAllowed(t *testing.T) {
	handler := NewRouter(Dependencies{Organizations: organization.NewHandler(&organizationApp{})})

	notFound := httptest.NewRecorder()
	handler.ServeHTTP(notFound, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("404 route status = %d", notFound.Code)
	}

	methodNotAllowed := httptest.NewRecorder()
	handler.ServeHTTP(methodNotAllowed, httptest.NewRequest(http.MethodDelete, "/api/v1/organizations", nil))
	if methodNotAllowed.Code != http.StatusMethodNotAllowed {
		t.Fatalf("405 route status = %d", methodNotAllowed.Code)
	}
}

func TestOrganizationHandlerMapsMissingRecordToNotFound(t *testing.T) {
	app := &organizationApp{err: pgx.ErrNoRows}
	handler := NewRouter(Dependencies{Organizations: organization.NewHandler(app)})
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+uuid.NewString(), nil))
	if res.Code != http.StatusNotFound {
		t.Fatalf("missing record status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestNewServerKeepsHTTPConfigurationSeparate(t *testing.T) {
	config := ServerConfig{Addr: ":8080", ReadHeaderTimeout: 5 * time.Second, IdleTimeout: time.Minute, MaxHeaderBytes: 4096}
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := NewServer(config, handler)
	if server.Addr != config.Addr || server.Handler == nil || server.ReadHeaderTimeout != config.ReadHeaderTimeout || server.IdleTimeout != config.IdleTimeout || server.MaxHeaderBytes != config.MaxHeaderBytes {
		t.Fatalf("server config not applied: %+v", server)
	}
}
