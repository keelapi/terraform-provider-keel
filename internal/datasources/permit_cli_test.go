package datasources_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

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

// fakePermits serves GET /v1/permits with Keel's filter validation: decision
// must be allow, deny, review or throttle and limit 1-200, else 400.
func fakePermits(t *testing.T) (*httptest.Server, func() []string) {
	t.Helper()
	fixture, err := os.ReadFile("testdata/permits_list.json")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var decisions []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		decision := r.URL.Query().Get("decision")
		mu.Lock()
		decisions = append(decisions, decision)
		mu.Unlock()
		switch decision {
		case "", "allow", "deny", "review", "throttle":
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"code":"invalid_request","message":"Invalid request payload.","field":"decision"}}`)
			return
		}
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), decisions...)
	}
}

func permitConfig(baseURL, arguments string) string {
	return fmt.Sprintf(`
provider "keel" {
  base_url = %q
  api_key  = "keel_sk_FakeClient_0123456789"
}

data "keel_permit" "test" {
%s
}
`, baseURL, arguments)
}

func TestPermitDataSourceReviewAndDeprecatedChallenge(t *testing.T) {
	requireTerraformCLI(t)
	srv, decisions := fakePermits(t)

	for _, decision := range []string{"review", "challenge"} {
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: permitConfig(srv.URL, fmt.Sprintf("  decision = %q\n  limit    = 5", decision)),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("data.keel_permit.test", "decision", decision),
						resource.TestCheckResourceAttr("data.keel_permit.test", "permits.#", "5"),
						resource.TestCheckResourceAttr("data.keel_permit.test", "permits.0.reason_code", "policy.rule_denied"),
						resource.TestCheckResourceAttr("data.keel_permit.test", "permits.0.outcome_kind", "decision"),
						resource.TestCheckResourceAttr("data.keel_permit.test", "permits.0.message", "Blocked by the production model rule."),
					),
				},
			},
		})
	}
	for _, sent := range decisions() {
		if sent != "review" {
			t.Errorf("sent decision=%q, want review", sent)
		}
	}
}

func TestPermitDataSourceValidatesFiltersAtPlan(t *testing.T) {
	requireTerraformCLI(t)
	srv, decisions := fakePermits(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      permitConfig(srv.URL, `  decision = "allowed"`),
				ExpectError: regexp.MustCompile(`must\s+be\s+one\s+of:\s+"allow",\s+"deny",\s+"review",\s+"throttle",\s+got:\s+"allowed"`),
			},
			{
				Config:      permitConfig(srv.URL, `  limit = 500`),
				ExpectError: regexp.MustCompile(`must\s+be\s+between\s+1\s+and\s+200,\s+got:\s+500`),
			},
		},
	})
	if got := decisions(); len(got) != 0 {
		t.Errorf("invalid filters reached the API: %v", got)
	}
}
