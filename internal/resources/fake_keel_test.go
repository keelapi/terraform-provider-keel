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
	fakeUserToken    = "dsess_FakeOrgOwnerSession"
	fakeOrgID        = "7f465e2c-8058-4173-80f3-472caed131c2"
	fakeAdminKey     = "keel_sk_FakeAdmin_0123456789"
	fakeProjectID    = "37fae9b0-d2eb-4f50-9650-5aa1197cc632"
	fakeAdminKeyID   = "fd6c1484-cda3-4c6d-a87f-2812b0d12510"
	fakeTimestampFmt = "2006-01-02T15:04:05Z"
)

// fakeKeel emulates the Keel API routes the provider calls, with the
// response shapes and authentication rules of the real API:
//
//   - /v1/api-keys routes accept only the admin API key;
//   - /v1/organizations/{org_id}/members routes accept only a user token, so
//     an API key gets the same 401; roles are stored lowercase;
//   - timestamps come back normalized to UTC "Z" form.
type fakeKeel struct {
	t   *testing.T
	srv *httptest.Server

	mu               sync.Mutex
	nextID           int
	apiKeys          []map[string]any
	pendingApprovals int
	members          []map[string]any
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

// addKey creates a key directly in the fake, as if made outside Terraform,
// and returns its ID.
func (f *fakeKeel) addKey(fields map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	record := map[string]any{
		"id":                 fmt.Sprintf("00000000-0000-4000-8000-%012d", f.nextID),
		"project_id":         fakeProjectID,
		"prefix":             fmt.Sprintf("keel_sk_%02d", f.nextID),
		"name":               nil,
		"description":        nil,
		"scope":              "admin",
		"created_by":         nil,
		"agent_principal_id": nil,
		"created_at":         "2026-09-21T09:30:00Z",
		"revoked_at":         nil,
		"last_used_at":       nil,
		"expires_at":         nil,
	}
	for k, v := range fields {
		record[k] = v
	}
	f.apiKeys = append(f.apiKeys, record)
	return record["id"].(string)
}

func (f *fakeKeel) providerConfig() string {
	return fmt.Sprintf(`
provider "keel" {
  base_url = %q
  api_key  = %q
}
`, f.srv.URL, fakeAdminKey)
}

func (f *fakeKeel) userTokenProviderConfig() string {
	return fmt.Sprintf(`
provider "keel" {
  base_url   = %q
  user_token = %q
}
`, f.srv.URL, fakeUserToken)
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
	case strings.HasPrefix(r.URL.Path, "/v1/organizations/"):
		if bearer != fakeUserToken {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid user token.")
			return
		}
		f.serveMembers(w, r)
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
		if scope == "approval" {
			// Enforced dual control: the key is minted only after approval.
			f.pendingApprovals++
			writeJSON(w, http.StatusAccepted, map[string]any{
				"project_id":                         fakeProjectID,
				"pending_change_id":                  fmt.Sprintf("00000000-0000-4000-9000-%012d", f.pendingApprovals),
				"status":                             "requested",
				"target_type":                        "approval_api_key_mint",
				"target_ref":                         "project:" + fakeProjectID + ":approval-api-key-mint",
				"proposed_change_hash":               "8ac30b04da34f8f2cabce2ef91cd47c91fd0a2b40a7525a202129b665452575a",
				"approval_requirement_snapshot_hash": "d6be301800b2ae773c3cfbba6ecc7a0b30bdc6ebc061a6c39211406dc011bf50",
				"expires_at":                         time.Now().Add(24 * time.Hour).UTC().Format(fakeTimestampFmt),
				"raw_key":                            "keel_sk_Ap_pending_secret",
				"prefix":                             "keel_sk_Ap",
				"scope":                              "approval",
			})
			return
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

func (f *fakeKeel) serveMembers(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/organizations/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] != fakeOrgID || parts[1] != "members" {
		writeError(w, http.StatusNotFound, "not_found", "Organization not found.")
		return
	}
	find := func(userID string) int {
		for i, m := range f.members {
			if m["user_id"] == userID {
				return i
			}
		}
		return -1
	}
	switch {
	case len(parts) == 2 && r.Method == http.MethodGet:
		items := f.members
		if items == nil {
			items = []map[string]any{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case len(parts) == 2 && r.Method == http.MethodPost:
		var req struct {
			UserID string `json:"user_id"`
			Role   string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request payload.")
			return
		}
		if find(req.UserID) >= 0 {
			writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "conflict", "message": "Organization member already exists.", "field": "user_id"}})
			return
		}
		f.nextID++
		member := map[string]any{
			"id":         fmt.Sprintf("00000000-0000-4000-a000-%012d", f.nextID),
			"org_id":     fakeOrgID,
			"user_id":    req.UserID,
			"role":       strings.ToLower(strings.TrimSpace(req.Role)),
			"created_at": time.Now().UTC().Format(fakeTimestampFmt),
		}
		f.members = append(f.members, member)
		writeJSON(w, http.StatusCreated, member)
	case len(parts) == 3 && r.Method == http.MethodPatch:
		i := find(parts[2])
		if i < 0 {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "not_found", "message": "Organization member not found.", "field": "user_id"}})
			return
		}
		var req struct {
			Role string `json:"role"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.members[i]["role"] = strings.ToLower(strings.TrimSpace(req.Role))
		writeJSON(w, http.StatusOK, f.members[i])
	case len(parts) == 3 && r.Method == http.MethodDelete:
		i := find(parts[2])
		if i < 0 {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "not_found", "message": "Organization member not found.", "field": "user_id"}})
			return
		}
		f.members = append(f.members[:i], f.members[i+1:]...)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "Method Not Allowed"})
	}
}

// checkMembersRemoved is a CheckDestroy for organization members.
func (f *fakeKeel) checkMembersRemoved(_ *terraform.State) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.members) > 0 {
		return fmt.Errorf("organization members still present after destroy: %v", f.members)
	}
	return nil
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
