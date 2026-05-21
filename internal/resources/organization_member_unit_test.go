package resources

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/keelapi/terraform-provider-keel/internal/client"
)

func TestOrganizationMemberResourceFindOrganizationMember(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/organizations/org_1/members" {
			t.Fatalf("path = %q, want /v1/organizations/org_1/members", r.URL.Path)
		}
		fmt.Fprint(w, `{"items":[{"id":"member_1","org_id":"org_1","user_id":"user_1","role":"member","created_at":"2026-01-01T00:00:00Z"}]}`)
	}))
	defer srv.Close()

	r := &organizationMemberResource{client: client.New(srv.URL, "test-key")}
	member, err := r.findOrganizationMember(context.Background(), "org_1", "user_1")
	if err != nil {
		t.Fatalf("findOrganizationMember returned error: %v", err)
	}
	if member == nil {
		t.Fatal("expected user_1 to be found")
	}
	if member.Role != "member" {
		t.Fatalf("Role = %q, want member", member.Role)
	}
}

func TestParseOrganizationMemberMutationPendingChange(t *testing.T) {
	_, pending, err := parseOrganizationMemberMutation([]byte(`{"id":"chg_1","status":"pending","target_type":"organization_member","target_ref":"user_1"}`))
	if err != nil {
		t.Fatalf("parseOrganizationMemberMutation returned error: %v", err)
	}
	if pending == nil {
		t.Fatal("expected pending change response")
	}
	if pending.ID != "chg_1" {
		t.Fatalf("pending ID = %q, want chg_1", pending.ID)
	}
}

func TestOrganizationMemberPaths(t *testing.T) {
	if got, want := organizationMembersPath("org_1"), "/v1/organizations/org_1/members"; got != want {
		t.Fatalf("organizationMembersPath = %q, want %q", got, want)
	}
	if got, want := organizationMemberPath("org_1", "user_1"), "/v1/organizations/org_1/members/user_1"; got != want {
		t.Fatalf("organizationMemberPath = %q, want %q", got, want)
	}
}
