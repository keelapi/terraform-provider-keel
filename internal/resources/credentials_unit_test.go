package resources

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

const (
	testOrgID  = "7f465e2c-8058-4173-80f3-472caed131c2"
	testUserID = "9605e229-53ce-4b24-9c01-e6e2057d202f"
)

func TestOrganizationMemberConfigureRequiresUserToken(t *testing.T) {
	r := &organizationMemberResource{}
	diags := configure(t, r, &client.ProviderData{APIKey: client.New("http://127.0.0.1:1", "keel_sk_admin")})
	if !diags.HasError() {
		t.Fatal("expected an error without a user token")
	}
	text := diagText(diags)
	if !strings.Contains(text, "API keys cannot manage organization membership; set KEEL_USER_TOKEN to a Keel user access token — short-lived") {
		t.Fatalf("diagnostic does not explain the missing user token:\n%s", text)
	}
	if r.client != nil {
		t.Fatal("resource must not fall back to the API key client")
	}
}

func TestOrganizationMemberConfigureUsesUserToken(t *testing.T) {
	r := &organizationMemberResource{}
	userClient := client.New("http://127.0.0.1:1", "dsess_user")
	diags := configure(t, r, &client.ProviderData{
		APIKey:    client.New("http://127.0.0.1:1", "keel_sk_admin"),
		UserToken: userClient,
	})
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if r.client != userClient {
		t.Fatal("organization member resource must use the user token client")
	}
}

func TestAPIKeyConfigureRequiresAPIKey(t *testing.T) {
	r := &apiKeyResource{}
	diags := configure(t, r, &client.ProviderData{UserToken: client.New("http://127.0.0.1:1", "dsess_user")})
	if !diags.HasError() {
		t.Fatal("expected an error without an API key")
	}
	if text := diagText(diags); !strings.Contains(text, "KEEL_API_KEY") {
		t.Fatalf("diagnostic does not name KEEL_API_KEY:\n%s", text)
	}
	if r.client != nil {
		t.Fatal("resource must not fall back to the user token client")
	}
}

func TestConfigureBeforeProviderIsConfigured(t *testing.T) {
	// Validation-only calls carry no provider data; that is not an error.
	for _, r := range []resource.ResourceWithConfigure{&apiKeyResource{}, &organizationMemberResource{}} {
		var resp resource.ConfigureResponse
		r.Configure(context.Background(), resource.ConfigureRequest{}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("%T: unexpected error: %v", r, resp.Diagnostics)
		}
	}
}

func TestOrganizationMemberRequestsSendUserToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer dsess_user" {
			t.Errorf("Authorization = %q, want the user token", got)
		}
		fmt.Fprintf(w, `{"items":[{"id":"0f3a1c52-8a3e-4c8e-9d1b-3f5b8c6f2a10","org_id":%q,"user_id":%q,"role":"member","created_at":"2026-09-20T12:00:00Z"}]}`, testOrgID, testUserID)
	}))
	defer srv.Close()

	r := &organizationMemberResource{}
	if diags := configure(t, r, &client.ProviderData{
		APIKey:    client.New(srv.URL, "keel_sk_admin"),
		UserToken: client.New(srv.URL, "dsess_user"),
	}); diags.HasError() {
		t.Fatalf("configure: %v", diags)
	}
	member, err := r.findOrganizationMember(context.Background(), testOrgID, testUserID)
	if err != nil {
		t.Fatalf("findOrganizationMember: %v", err)
	}
	if member == nil || member.Role != "member" {
		t.Fatalf("member = %+v, want role member", member)
	}
}

func TestOrganizationMemberReadExplainsRejectedUserToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"code":"unauthorized","message":"Missing or invalid user token."}}`)
	}))
	defer srv.Close()

	r := &organizationMemberResource{client: client.New(srv.URL, "dsess_expired")}
	s := resourceSchema(t, r)
	state := testState(t, s, map[string]tftypes.Value{
		"id":      tfString("0f3a1c52-8a3e-4c8e-9d1b-3f5b8c6f2a10"),
		"org_id":  tfString(testOrgID),
		"user_id": tfString(testUserID),
		"role":    tfString("member"),
	})
	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for a rejected user token")
	}
	text := diagText(resp.Diagnostics)
	for _, want := range []string{"unauthorized: Missing or invalid user token.", "User tokens are short-lived"} {
		if !strings.Contains(text, want) {
			t.Errorf("diagnostic missing %q:\n%s", want, text)
		}
	}
}

func TestUserTokenErrorDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/401":
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"code":"unauthorized","message":"Missing or invalid user token."}}`)
		case "/429-auth":
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"code":"auth_failure_rate_limited","message":"Too many authentication failures.","retry_after_seconds":38,"scope":"dashboard_auth"}}`)
		case "/403":
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"error":{"code":"forbidden","message":"Forbidden."}}`)
		}
	}))
	defer srv.Close()

	c := client.New(srv.URL, "dsess_expired")
	for path, wantHint := range map[string]bool{"/401": true, "/429-auth": true, "/403": false} {
		_, err := c.Get(context.Background(), path)
		if err == nil {
			t.Fatalf("%s: expected an error", path)
		}
		detail := userTokenErrorDetail(err)
		if !strings.HasPrefix(detail, err.Error()) {
			t.Errorf("%s: detail %q does not start with the API error", path, detail)
		}
		if got := strings.Contains(detail, rejectedUserTokenHint); got != wantHint {
			t.Errorf("%s: hint present = %v, want %v", path, got, wantHint)
		}
	}
}
