package resources

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/keelapi/terraform-provider-keel/internal/client"
)

func TestAPIKeyResourceFindAPIKeyUsesAPIKeyListEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api-keys" {
			t.Fatalf("path = %q, want /v1/api-keys", r.URL.Path)
		}
		if got := r.URL.Query().Get("status"); got != "all" {
			t.Fatalf("status query = %q, want all", got)
		}
		if got := r.URL.Query().Get("limit"); got != "200" {
			t.Fatalf("limit query = %q, want 200", got)
		}

		switch r.URL.Query().Get("cursor") {
		case "":
			fmt.Fprint(w, `{"items":[{"id":"key_1","project_id":"proj_1","scope":"client"}],"next_cursor":"next"}`)
		case "next":
			fmt.Fprint(w, `{"items":[{"id":"key_2","project_id":"proj_1","scope":"admin","prefix":"keel_live"}],"next_cursor":null}`)
		default:
			t.Fatalf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer srv.Close()

	r := &apiKeyResource{client: client.New(srv.URL, "test-key")}
	key, err := r.findAPIKey(context.Background(), "key_2", "")
	if err != nil {
		t.Fatalf("findAPIKey returned error: %v", err)
	}
	if key == nil {
		t.Fatal("expected key_2 to be found")
	}
	if key.ProjectID != "proj_1" {
		t.Fatalf("ProjectID = %q, want proj_1", key.ProjectID)
	}
	if key.Scope != "admin" {
		t.Fatalf("Scope = %q, want admin", key.Scope)
	}
}

func TestAPIKeyResourceFindAPIKeyUsesProjectAPIKeyListWhenProjectIDSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/proj_1/api-keys" {
			t.Fatalf("path = %q, want /v1/projects/proj_1/api-keys", r.URL.Path)
		}
		fmt.Fprint(w, `{"items":[{"id":"key_1","project_id":"proj_1","scope":"client"}],"next_cursor":null}`)
	}))
	defer srv.Close()

	r := &apiKeyResource{client: client.New(srv.URL, "test-key")}
	key, err := r.findAPIKey(context.Background(), "key_1", "proj_1")
	if err != nil {
		t.Fatalf("findAPIKey returned error: %v", err)
	}
	if key == nil {
		t.Fatal("expected key_1 to be found")
	}
	if key.ProjectID != "proj_1" {
		t.Fatalf("ProjectID = %q, want proj_1", key.ProjectID)
	}
}

func TestAPIKeyPaths(t *testing.T) {
	if got, want := apiKeyCollectionPath(""), "/v1/api-keys"; got != want {
		t.Fatalf("apiKeyCollectionPath empty = %q, want %q", got, want)
	}
	if got, want := apiKeyCollectionPath("proj_1"), "/v1/projects/proj_1/api-keys"; got != want {
		t.Fatalf("apiKeyCollectionPath project = %q, want %q", got, want)
	}
	if got, want := apiKeyRevokePath("", "key_1"), "/v1/api-keys/key_1/revoke"; got != want {
		t.Fatalf("apiKeyRevokePath empty = %q, want %q", got, want)
	}
	if got, want := apiKeyRevokePath("proj_1", "key_1"), "/v1/projects/proj_1/api-keys/key_1/revoke"; got != want {
		t.Fatalf("apiKeyRevokePath project = %q, want %q", got, want)
	}
}
