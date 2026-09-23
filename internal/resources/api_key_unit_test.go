package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

const (
	testProjectID      = "37fae9b0-d2eb-4f50-9650-5aa1197cc632"
	testOtherProjectID = "179b3c38-8ff4-42f6-a4f9-05c1681ed6bd"
	testProviderKeyID  = "fd6c1484-cda3-4c6d-a87f-2812b0d12510"
	testKeyID          = "a9c8b5b1-b657-4688-9bf3-94db0f3e55d8"
)

// apiKeyRecord renders an ApiKeyRecord as the Keel API does.
func apiKeyRecord(id, projectID, scope, revokedAt string) map[string]any {
	record := map[string]any{
		"id":                 id,
		"project_id":         projectID,
		"prefix":             "keel_sk_IG",
		"name":               "backend-service",
		"description":        nil,
		"scope":              scope,
		"created_by":         nil,
		"agent_principal_id": nil,
		"created_at":         "2026-09-23T03:45:55Z",
		"revoked_at":         nil,
		"last_used_at":       nil,
		"expires_at":         nil,
	}
	if revokedAt != "" {
		record["revoked_at"] = revokedAt
	}
	return record
}

// fakeAPIKeysServer serves GET /v1/api-keys and POST /v1/api-keys/{id}/revoke
// from records. Unexpected routes fail the test.
type fakeAPIKeysServer struct {
	t       *testing.T
	mu      sync.Mutex
	records []map[string]any
	paths   []string
}

func (f *fakeAPIKeysServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paths = append(f.paths, r.Method+" "+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/api-keys":
		if got := r.URL.Query().Get("status"); got != "all" {
			f.t.Errorf("status query = %q, want all", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": f.records, "next_cursor": nil})
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/api-keys/") && strings.HasSuffix(r.URL.Path, "/revoke"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/api-keys/"), "/revoke")
		for _, record := range f.records {
			if record["id"] == id {
				record["revoked_at"] = "2026-09-23T04:00:00Z"
				_ = json.NewEncoder(w).Encode(record)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"code":"not_found","message":"API key not found."}}`)
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"detail":"Not Found"}`)
	}
}

func (f *fakeAPIKeysServer) requested() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.paths...)
}

func newFakeAPIKeys(t *testing.T, records ...map[string]any) (*fakeAPIKeysServer, *apiKeyResource) {
	t.Helper()
	fake := &fakeAPIKeysServer{t: t, records: records}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	return fake, &apiKeyResource{client: client.New(srv.URL, "keel_sk_admin")}
}

// apiKeyState is the state keel_api_key holds after a create through
// /v1/api-keys: project_id is always recorded.
func apiKeyState(t *testing.T, r *apiKeyResource, id, projectID string) map[string]tftypes.Value {
	t.Helper()
	return map[string]tftypes.Value{
		"id":         tfString(id),
		"project_id": tfString(projectID),
		"name":       tfString("backend-service"),
		"scope":      tfString("client"),
		"prefix":     tfString("keel_sk_IG"),
		"raw_key":    tfString("keel_sk_IG_secret"),
		"created_at": tfString("2026-09-23T03:45:55Z"),
		"status":     tfString("active"),
	}
}

func readAPIKey(t *testing.T, r *apiKeyResource, attrs map[string]tftypes.Value) resource.ReadResponse {
	t.Helper()
	s := resourceSchema(t, r)
	state := testState(t, s, attrs)
	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
	return resp
}

func deleteAPIKey(t *testing.T, r *apiKeyResource, attrs map[string]tftypes.Value) resource.DeleteResponse {
	t.Helper()
	s := resourceSchema(t, r)
	state := testState(t, s, attrs)
	resp := resource.DeleteResponse{State: state}
	r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
	return resp
}

func TestAPIKeyReadUsesAPIKeysRouteWhenStateHasProjectID(t *testing.T) {
	fake, r := newFakeAPIKeys(t,
		apiKeyRecord(testProviderKeyID, testProjectID, "admin", ""),
		apiKeyRecord(testKeyID, testProjectID, "client", ""),
	)

	resp := readAPIKey(t, r, apiKeyState(t, r, testKeyID, testProjectID))
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", resp.Diagnostics)
	}
	if got, ok := stateString(t, resp.State, "project_id"); !ok || got != testProjectID {
		t.Fatalf("project_id = %q, want %q", got, testProjectID)
	}
	if got, _ := stateString(t, resp.State, "raw_key"); got != "keel_sk_IG_secret" {
		t.Fatalf("raw_key = %q, want the create-time secret kept", got)
	}
	for _, p := range fake.requested() {
		if p != "GET /v1/api-keys" {
			t.Fatalf("Read requested %s, want only GET /v1/api-keys", p)
		}
	}
}

func TestAPIKeyReadRemovesRevokedOrMissingKey(t *testing.T) {
	for name, records := range map[string][]map[string]any{
		"revoked": {
			apiKeyRecord(testProviderKeyID, testProjectID, "admin", ""),
			apiKeyRecord(testKeyID, testProjectID, "client", "2026-09-23T04:00:00Z"),
		},
		"missing": {
			apiKeyRecord(testProviderKeyID, testProjectID, "admin", ""),
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, r := newFakeAPIKeys(t, records...)
			resp := readAPIKey(t, r, apiKeyState(t, r, testKeyID, testProjectID))
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read returned errors: %v", resp.Diagnostics)
			}
			if !resp.State.Raw.IsNull() {
				t.Fatal("expected the key to be removed from state")
			}
		})
	}
}

func TestAPIKeyReadRefusesKeyFromAnotherProject(t *testing.T) {
	// The provider's API key belongs to another project, so the key cannot be
	// listed. It must not be silently dropped from state.
	_, r := newFakeAPIKeys(t, apiKeyRecord(testProviderKeyID, testOtherProjectID, "admin", ""))
	resp := readAPIKey(t, r, apiKeyState(t, r, testKeyID, testProjectID))
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for a key recorded in another project")
	}
	if text := diagText(resp.Diagnostics); !strings.Contains(text, testOtherProjectID) {
		t.Fatalf("diagnostic does not name the provider key's project:\n%s", text)
	}
}

func TestAPIKeyDeleteUsesAPIKeysRevokeRoute(t *testing.T) {
	fake, r := newFakeAPIKeys(t,
		apiKeyRecord(testProviderKeyID, testProjectID, "admin", ""),
		apiKeyRecord(testKeyID, testProjectID, "client", ""),
	)
	resp := deleteAPIKey(t, r, apiKeyState(t, r, testKeyID, testProjectID))
	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete returned errors: %v", resp.Diagnostics)
	}
	want := "POST /v1/api-keys/" + testKeyID + "/revoke"
	if got := fake.requested(); len(got) != 1 || got[0] != want {
		t.Fatalf("Delete requested %v, want [%s]", got, want)
	}
}

func TestAPIKeyDeleteNotFound(t *testing.T) {
	t.Run("same project: already gone", func(t *testing.T) {
		_, r := newFakeAPIKeys(t, apiKeyRecord(testProviderKeyID, testProjectID, "admin", ""))
		if resp := deleteAPIKey(t, r, apiKeyState(t, r, testKeyID, testProjectID)); resp.Diagnostics.HasError() {
			t.Fatalf("Delete returned errors: %v", resp.Diagnostics)
		}
	})
	t.Run("other project: error", func(t *testing.T) {
		_, r := newFakeAPIKeys(t, apiKeyRecord(testProviderKeyID, testOtherProjectID, "admin", ""))
		resp := deleteAPIKey(t, r, apiKeyState(t, r, testKeyID, testProjectID))
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error: the key lives in another project and was not revoked")
		}
	})
}

func TestAPIKeyImportState(t *testing.T) {
	r := &apiKeyResource{}
	s := resourceSchema(t, r)
	for _, tc := range []struct {
		id        string
		wantID    string
		wantProj  string
		wantError bool
	}{
		{id: testKeyID, wantID: testKeyID},
		{id: testProjectID + "/" + testKeyID, wantID: testKeyID, wantProj: testProjectID},
		{id: "a/b/c", wantError: true},
		{id: testProjectID + "/", wantError: true},
	} {
		t.Run(tc.id, func(t *testing.T) {
			resp := resource.ImportStateResponse{State: emptyState(t, s)}
			r.ImportState(context.Background(), resource.ImportStateRequest{ID: tc.id}, &resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("HasError = %v, want %v: %v", resp.Diagnostics.HasError(), tc.wantError, resp.Diagnostics)
			}
			if tc.wantError {
				return
			}
			if got, _ := stateString(t, resp.State, "id"); got != tc.wantID {
				t.Errorf("id = %q, want %q", got, tc.wantID)
			}
			if got, _ := stateString(t, resp.State, "project_id"); got != tc.wantProj {
				t.Errorf("project_id = %q, want %q", got, tc.wantProj)
			}
		})
	}
}

func TestAPIKeyImportedProjectMustMatch(t *testing.T) {
	_, r := newFakeAPIKeys(t,
		apiKeyRecord(testProviderKeyID, testProjectID, "admin", ""),
		apiKeyRecord(testKeyID, testProjectID, "client", ""),
	)
	resp := readAPIKey(t, r, map[string]tftypes.Value{
		"id":         tfString(testKeyID),
		"project_id": tfString(testOtherProjectID),
	})
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the import ID names another project")
	}
}

func TestAPIKeyProjectIDIsComputedOnly(t *testing.T) {
	attr := resourceSchema(t, &apiKeyResource{}).Attributes["project_id"]
	if !attr.IsComputed() || attr.IsOptional() || attr.IsRequired() {
		t.Fatalf("project_id: computed=%v optional=%v required=%v, want computed only",
			attr.IsComputed(), attr.IsOptional(), attr.IsRequired())
	}
}

func TestAPIKeyResourceFindAPIKeyPaginates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api-keys" {
			t.Fatalf("path = %q, want /v1/api-keys", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "200" {
			t.Fatalf("limit query = %q, want 200", got)
		}
		switch r.URL.Query().Get("cursor") {
		case "":
			fmt.Fprintf(w, `{"items":[{"id":%q,"project_id":%q,"prefix":"keel_sk_Ab","scope":"admin","created_at":"2026-09-20T12:00:00Z"}],"next_cursor":"2026-09-20T12:00:00+00:00|%s"}`, testProviderKeyID, testProjectID, testProviderKeyID)
		case "2026-09-20T12:00:00+00:00|" + testProviderKeyID:
			fmt.Fprintf(w, `{"items":[{"id":%q,"project_id":%q,"prefix":"keel_sk_IG","scope":"client","created_at":"2026-09-19T12:00:00Z"}],"next_cursor":null}`, testKeyID, testProjectID)
		default:
			t.Fatalf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer srv.Close()

	r := &apiKeyResource{client: client.New(srv.URL, "keel_sk_admin")}
	key, listedProjectID, err := r.findAPIKey(context.Background(), testKeyID)
	if err != nil {
		t.Fatalf("findAPIKey returned error: %v", err)
	}
	if key == nil || key.Scope != "client" {
		t.Fatalf("key = %+v, want the client key from the second page", key)
	}
	if listedProjectID != testProjectID {
		t.Fatalf("listedProjectID = %q, want %q", listedProjectID, testProjectID)
	}
}

func TestAPIKeyRevokePath(t *testing.T) {
	if got, want := apiKeyRevokePath(testKeyID), "/v1/api-keys/"+testKeyID+"/revoke"; got != want {
		t.Fatalf("apiKeyRevokePath = %q, want %q", got, want)
	}
}
