package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

// Response bodies as the Keel API sends them for POST /v1/api-keys with
// scope "approval".
const (
	pendingApprovalKeySecret = "keel_sk_U_pending_secret_do_not_store"
	pendingApprovalKeyBody   = `{"project_id":"179b3c38-8ff4-42f6-a4f9-05c1681ed6bd","pending_change_id":"eebf2bfc-b129-4456-a1a4-fe2f66d443ad","status":"requested","target_type":"approval_api_key_mint","target_ref":"project:179b3c38-8ff4-42f6-a4f9-05c1681ed6bd:approval-api-key-mint","proposed_change_hash":"8ac30b04da34f8f2cabce2ef91cd47c91fd0a2b40a7525a202129b665452575a","approval_requirement_snapshot_hash":"d6be301800b2ae773c3cfbba6ecc7a0b30bdc6ebc061a6c39211406dc011bf50","expires_at":"2026-09-24T03:45:55Z","raw_key":"` + pendingApprovalKeySecret + `","prefix":"keel_sk_U_","scope":"approval"}`
	approvalForbiddenBody    = `{"error":{"code":"approval.authority_configuration_required","message":"Approval credentials require independently approved authority configuration."}}`
)

func createAPIKey(t *testing.T, serverURL string, attrs map[string]tftypes.Value) resource.CreateResponse {
	t.Helper()
	r := &apiKeyResource{client: client.New(serverURL, "keel_sk_admin")}
	s := resourceSchema(t, r)
	resp := resource.CreateResponse{State: emptyState(t, s)}
	r.Create(context.Background(), resource.CreateRequest{Plan: testPlan(t, s, attrs)}, &resp)
	return resp
}

func approvalScopePlan() map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"name":  tfString("approver-key"),
		"scope": tfString("approval"),
	}
}

func TestAPIKeyCreatePendingApprovalIsNotStored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		if req["scope"] != "approval" {
			t.Errorf("request scope = %v, want approval", req["scope"])
		}
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, pendingApprovalKeyBody)
	}))
	defer srv.Close()

	resp := createAPIKey(t, srv.URL, approvalScopePlan())
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for a pending approval-scope key")
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("a pending approval-scope key must not be written to state")
	}
	text := diagText(resp.Diagnostics)
	for _, want := range []string{"eebf2bfc-b129-4456-a1a4-fe2f66d443ad", "2026-09-24T03:45:55Z", "dual-control approval"} {
		if !strings.Contains(text, want) {
			t.Errorf("diagnostic missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, pendingApprovalKeySecret) {
		t.Fatal("diagnostic must not reveal the pending key's secret")
	}
}

func TestAPIKeyCreatePendingDetectedFromBodyAlone(t *testing.T) {
	// Even if a proxy rewrote the 202 to 201, a pending body is not a key.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, pendingApprovalKeyBody)
	}))
	defer srv.Close()

	resp := createAPIKey(t, srv.URL, approvalScopePlan())
	if !resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
		t.Fatalf("expected an error and no state, got diagnostics %v", resp.Diagnostics)
	}
}

func TestAPIKeyCreateApprovalForbiddenShowsAPIMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, approvalForbiddenBody)
	}))
	defer srv.Close()

	resp := createAPIKey(t, srv.URL, approvalScopePlan())
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for a forbidden approval-scope key")
	}
	text := diagText(resp.Diagnostics)
	for _, want := range []string{
		"approval.authority_configuration_required: Approval credentials require independently approved authority configuration.",
		"enforces dual control",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("diagnostic missing %q:\n%s", want, text)
		}
	}
}

func TestAPIKeyCreateWithoutKeyIDIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer srv.Close()

	resp := createAPIKey(t, srv.URL, map[string]tftypes.Value{"scope": tfString("client")})
	if !resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
		t.Fatalf("expected an error and no state for a response without an id, got %v", resp.Diagnostics)
	}
}

func TestAPIKeyCreateStoresKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api-keys" {
			t.Errorf("path = %q, want /v1/api-keys", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"a9c8b5b1-b657-4688-9bf3-94db0f3e55d8","project_id":"37fae9b0-d2eb-4f50-9650-5aa1197cc632","prefix":"keel_sk_IG","name":"backend-service","description":null,"scope":"client","created_by":null,"agent_principal_id":null,"created_at":"2026-09-23T03:45:55Z","revoked_at":null,"last_used_at":null,"expires_at":null,"raw_key":"keel_sk_IG_secret"}`)
	}))
	defer srv.Close()

	resp := createAPIKey(t, srv.URL, map[string]tftypes.Value{
		"name":  tfString("backend-service"),
		"scope": tfString("client"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create returned errors: %v", resp.Diagnostics)
	}
	for name, want := range map[string]string{
		"id":         "a9c8b5b1-b657-4688-9bf3-94db0f3e55d8",
		"project_id": "37fae9b0-d2eb-4f50-9650-5aa1197cc632",
		"raw_key":    "keel_sk_IG_secret",
		"status":     "active",
	} {
		if got, _ := stateString(t, resp.State, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
