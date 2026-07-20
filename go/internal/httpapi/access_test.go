package httpapi

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ml-ai-ops/platform/internal/auth"
	"github.com/ml-ai-ops/platform/internal/store"
	"github.com/ml-ai-ops/platform/pkg/api"
)

func accessServer(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	repository := store.New()
	_, err := repository.UpsertUserAccess("user-1", api.UpsertUserAccessRequest{
		Email: "user@example.com", Role: "user",
		Services:   []string{"overview", "projects", "pipelines"},
		ProjectIDs: []string{"prj-demo"},
		Storage:    api.StorageGrant{SizeGB: 10, Buckets: []string{"user-data"}},
		Compute:    api.ComputeGrant{VCPUs: 2, MemoryGB: 4, MaxVMs: 1, MaxProjects: 1, MaxRuns: 1},
	}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	static, _ := fs.Sub(fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}, ".")
	return New(repository, static), repository
}

func userRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	principal := auth.Principal{Subject: "user-1", Email: "user@example.com", Roles: []string{auth.RoleUser}}
	return request.WithContext(auth.WithPrincipal(request.Context(), principal))
}

func TestProvisionedUserIsDenyByDefaultOutsideAssignedServices(t *testing.T) {
	server, _ := accessServer(t)
	for _, test := range []struct {
		path string
		want int
	}{
		{"/api/v1/projects", http.StatusOK},
		{"/api/v1/models", http.StatusForbidden},
		{"/api/v1/components", http.StatusForbidden},
		{"/api/v1/admin/users", http.StatusForbidden},
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, userRequest(http.MethodGet, test.path, ""))
		if response.Code != test.want {
			t.Errorf("GET %s = %d, want %d: %s", test.path, response.Code, test.want, response.Body.String())
		}
	}
}

func TestLegacyReadOnlyAndServiceRolesCannotReadAdminProfiles(t *testing.T) {
	for _, role := range []string{auth.RoleViewer, auth.RoleEngineer, auth.RoleService} {
		t.Run(role, func(t *testing.T) {
			if auth.Allowed(auth.Principal{Roles: []string{role}}, http.MethodGet, "/api/v1/admin/users") {
				t.Fatalf("%s must not read user access profiles", role)
			}
		})
	}
}

func TestNormalUserProjectOwnershipAndQuota(t *testing.T) {
	server, _ := accessServer(t)
	first := httptest.NewRecorder()
	server.ServeHTTP(first, userRequest(http.MethodPost, "/api/v1/projects", `{"name":"owned project","template":"blank"}`))
	if first.Code != http.StatusCreated || !strings.Contains(first.Body.String(), `"owner_subject":"user-1"`) {
		t.Fatalf("first project: %d %s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	server.ServeHTTP(second, userRequest(http.MethodPost, "/api/v1/projects", `{"name":"over quota","template":"blank"}`))
	if second.Code != http.StatusForbidden || !strings.Contains(second.Body.String(), "quota") {
		t.Fatalf("second project must be denied: %d %s", second.Code, second.Body.String())
	}
}

func TestNormalUserNeedsGitServiceToCreateConnectedProject(t *testing.T) {
	server, _ := accessServer(t)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, userRequest(http.MethodPost, "/api/v1/projects", `{"name":"connected project","template":"blank","repository_url":"https://github.com/acme/project.git"}`))
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "Git service") {
		t.Fatalf("repository binding must require Git service: %d %s", response.Code, response.Body.String())
	}
}

func TestDisabledUserIsDeniedEvenForAssignedService(t *testing.T) {
	server, repository := accessServer(t)
	_, err := repository.UpsertUserAccess("user-1", api.UpsertUserAccessRequest{
		Role: "user", Services: []string{"projects"}, Disabled: true,
	}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, userRequest(http.MethodGet, "/api/v1/projects", ""))
	if response.Code != http.StatusForbidden {
		t.Fatalf("disabled user must be denied, got %d", response.Code)
	}
}

func TestAdminCanProvisionAndRevokeUser(t *testing.T) {
	server := testServer()
	put := httptest.NewRecorder()
	server.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/new-user", strings.NewReader(
		`{"email":"new@example.com","role":"user","services":["projects"],"storage":{"size_gb":5},"compute":{"vcpus":1,"memory_gb":2,"max_projects":1}}`,
	)))
	if put.Code != http.StatusOK {
		t.Fatalf("provision failed: %d %s", put.Code, put.Body.String())
	}
	remove := httptest.NewRecorder()
	server.ServeHTTP(remove, httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/new-user", nil))
	if remove.Code != http.StatusNoContent {
		t.Fatalf("revoke failed: %d %s", remove.Code, remove.Body.String())
	}
}

func TestUserCanRequestAccessAndAdminCanReview(t *testing.T) {
	server, _ := accessServer(t)
	create := httptest.NewRecorder()
	server.ServeHTTP(create, userRequest(http.MethodPost, "/api/v1/access-requests", `{"reason":"Need model registry for churn delivery","requested_services":["models","catalog"]}`))
	if create.Code != http.StatusCreated {
		t.Fatalf("request access failed: %d %s", create.Code, create.Body.String())
	}
	var created api.AccessRequest
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	mine := httptest.NewRecorder()
	server.ServeHTTP(mine, userRequest(http.MethodGet, "/api/v1/access-requests", ""))
	if mine.Code != http.StatusOK || !strings.Contains(mine.Body.String(), created.ID) {
		t.Fatalf("own requests missing: %d %s", mine.Code, mine.Body.String())
	}
	review := httptest.NewRecorder()
	server.ServeHTTP(review, httptest.NewRequest(http.MethodPatch, "/api/v1/admin/access-requests/"+created.ID, strings.NewReader(`{"status":"approved","note":"Capacity confirmed"}`)))
	if review.Code != http.StatusOK || !strings.Contains(review.Body.String(), `"status":"approved"`) {
		t.Fatalf("review failed: %d %s", review.Code, review.Body.String())
	}
}

func TestPersonalAPITokenAPIHidesHashAndRevokes(t *testing.T) {
	server := testServer()
	create := httptest.NewRecorder()
	server.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/v1/settings/tokens", strings.NewReader(
		`{"name":"laptop CLI","services":["projects"],"project_ids":["prj-demo"],"expires_in_days":30}`,
	)))
	if create.Code != http.StatusCreated || !strings.Contains(create.Body.String(), `"secret":"nxs_`) ||
		strings.Contains(create.Body.String(), "secret_hash") {
		t.Fatalf("create token: %d %s", create.Code, create.Body.String())
	}
	var created api.CreatedAPIToken
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/settings/tokens", nil))
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), "secret_hash") {
		t.Fatalf("list token: %d %s", list.Code, list.Body.String())
	}
	revoke := httptest.NewRecorder()
	server.ServeHTTP(revoke, httptest.NewRequest(http.MethodDelete, "/api/v1/settings/tokens/"+created.Token.ID, nil))
	if revoke.Code != http.StatusNoContent {
		t.Fatalf("revoke token: %d %s", revoke.Code, revoke.Body.String())
	}
}
