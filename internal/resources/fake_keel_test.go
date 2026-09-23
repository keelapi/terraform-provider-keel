package resources_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// requireTerraformCLI skips unless a Terraform CLI is available locally, so
// these tests never download one. They drive the real CLI against the
// in-process provider and a fake Keel API; nothing leaves the machine.
func requireTerraformCLI(t *testing.T) {
	t.Helper()
	path := os.Getenv("TF_ACC_TERRAFORM_PATH")
	if path == "" {
		found, err := exec.LookPath("terraform")
		if err != nil {
			t.Skip("terraform CLI not found on PATH; set TF_ACC_TERRAFORM_PATH to run this test")
		}
		path = found
	}
	t.Setenv("TF_ACC_TERRAFORM_PATH", path)
	t.Setenv("CHECKPOINT_DISABLE", "1")
	for _, name := range []string{"KEEL_BASE_URL", "KEEL_API_KEY", "KEEL_USER_TOKEN"} {
		t.Setenv(name, "")
	}
}

const (
	fakeAdminKey     = "keel_sk_FakeAdmin_0123456789"
	fakeProjectID    = "37fae9b0-d2eb-4f50-9650-5aa1197cc632"
	fakeAdminKeyID   = "fd6c1484-cda3-4c6d-a87f-2812b0d12510"
	fakeTimestampFmt = "2006-01-02T15:04:05Z"
)

// fakeKeel emulates the Keel API routes the provider calls, with the
// response shapes and authentication rules of the real API:
//
//   - /v1/api-keys routes accept only the admin API key;
//   - /v1/projects/{id}/api-keys routes accept only a user token, so an API
//     key gets 401 "Missing or invalid user token.";
//   - timestamps come back normalized to UTC "Z" form.
type fakeKeel struct {
	t   *testing.T
	srv *httptest.Server

	mu      sync.Mutex
	nextID  int
	apiKeys []map[string]any
}

func newFakeKeel(t *testing.T) *fakeKeel {
	t.Helper()
	f := &fakeKeel{t: t}
	f.apiKeys = []map[string]any{{
		"id": fakeAdminKeyID, "project_id": fakeProjectID, "prefix": "keel_sk_Fa",
		"name": "tf-admin", "description": nil, "scope": "admin", "created_by": nil,
		"agent_principal_id": nil, "created_at": "2026-09-20T12:00:00Z",
		"revoked_at": nil, "last_used_at": nil, "expires_at": nil,
	}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeKeel) URL() string { return f.srv.URL }

func (f *fakeKeel) providerConfig() string {
	return fmt.Sprintf(`
provider "keel" {
  base_url = %q
  api_key  = %q
}
`, f.srv.URL, fakeAdminKey)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func (f *fakeKeel) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	switch {
	case strings.HasPrefix(r.URL.Path, "/v1/projects/"):
		writeError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid user token.")
	case r.URL.Path == "/v1/api-keys" || strings.HasPrefix(r.URL.Path, "/v1/api-keys/"):
		if bearer != fakeAdminKey {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid API key.")
			return
		}
		f.serveAPIKeys(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
	}
}

func (f *fakeKeel) serveAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/api-keys":
		var req struct {
			Name             *string `json:"name"`
			Description      *string `json:"description"`
			Scope            *string `json:"scope"`
			AgentPrincipalID *string `json:"agent_principal_id"`
			ExpiresAt        *string `json:"expires_at"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields() // the API rejects unknown fields
		if err := dec.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "invalid_request", "message": "Invalid request payload."}})
			return
		}
		scope := "admin"
		if req.Scope != nil {
			scope = *req.Scope
		}
		var expiresAt any
		if req.ExpiresAt != nil {
			ts, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "invalid_request", "message": "Invalid request payload.", "field": "expires_at"}})
				return
			}
			expiresAt = ts.UTC().Format(fakeTimestampFmt)
		}
		f.nextID++
		record := map[string]any{
			"id":                 fmt.Sprintf("00000000-0000-4000-8000-%012d", f.nextID),
			"project_id":         fakeProjectID,
			"prefix":             fmt.Sprintf("keel_sk_%02d", f.nextID),
			"name":               req.Name,
			"description":        req.Description,
			"scope":              scope,
			"created_by":         nil,
			"agent_principal_id": req.AgentPrincipalID,
			"created_at":         time.Now().UTC().Format(fakeTimestampFmt),
			"revoked_at":         nil,
			"last_used_at":       nil,
			"expires_at":         expiresAt,
		}
		f.apiKeys = append(f.apiKeys, record)
		created := map[string]any{"raw_key": fmt.Sprintf("keel_sk_%02d_secret", f.nextID)}
		for k, v := range record {
			created[k] = v
		}
		writeJSON(w, http.StatusCreated, created)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/api-keys":
		items := make([]map[string]any, 0, len(f.apiKeys))
		for i := len(f.apiKeys) - 1; i >= 0; i-- {
			record := f.apiKeys[i]
			if r.URL.Query().Get("status") != "all" && record["revoked_at"] != nil {
				continue
			}
			items = append(items, record)
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/revoke"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/api-keys/"), "/revoke")
		for _, record := range f.apiKeys {
			if record["id"] == id {
				if record["revoked_at"] == nil {
					record["revoked_at"] = time.Now().UTC().Format(fakeTimestampFmt)
				}
				writeJSON(w, http.StatusOK, record)
				return
			}
		}
		writeError(w, http.StatusNotFound, "not_found", "API key not found.")
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "Method Not Allowed"})
	}
}

// checkCreatedKeysRevoked is a CheckDestroy: every key Terraform created must
// have been revoked through the API.
func (f *fakeKeel) checkCreatedKeysRevoked(_ *terraform.State) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	var live bytes.Buffer
	for _, record := range f.apiKeys {
		if record["id"] != fakeAdminKeyID && record["revoked_at"] == nil {
			fmt.Fprintf(&live, " %v", record["id"])
		}
	}
	if live.Len() > 0 {
		return fmt.Errorf("API keys still active after destroy:%s", live.String())
	}
	return nil
}
