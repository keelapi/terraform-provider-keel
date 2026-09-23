package resources_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAPIKeyLifecycleWithAdminAPIKey drives create, the post-apply plan,
// import and destroy through the Terraform CLI with an admin API key: every
// step must stay on the /v1/api-keys routes that accept it.
func TestAPIKeyLifecycleWithAdminAPIKey(t *testing.T) {
	requireTerraformCLI(t)
	fake := newFakeKeel(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             fake.checkCreatedKeysRevoked,
		Steps: []resource.TestStep{
			{
				Config: fake.providerConfig() + `
resource "keel_api_key" "test" {
  name  = "tf-e2e-client"
  scope = "client"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("keel_api_key.test", "id"),
					resource.TestCheckResourceAttr("keel_api_key.test", "project_id", fakeProjectID),
					resource.TestCheckResourceAttr("keel_api_key.test", "scope", "client"),
					resource.TestCheckResourceAttr("keel_api_key.test", "status", "active"),
					resource.TestCheckResourceAttrSet("keel_api_key.test", "raw_key"),
				),
			},
			{
				ResourceName:            "keel_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"raw_key"},
			},
		},
	})
}

// TestAPIKeyApprovalScopePendingIsAnError: under enforced dual control Keel
// answers 202 with a pending change. Apply must fail and store nothing.
func TestAPIKeyApprovalScopePendingIsAnError(t *testing.T) {
	requireTerraformCLI(t)
	fake := newFakeKeel(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             fake.checkCreatedKeysRevoked,
		Steps: []resource.TestStep{
			{
				Config: fake.providerConfig() + `
resource "keel_api_key" "approver" {
  name  = "approver"
  scope = "approval"
}
`,
				ExpectError: regexp.MustCompile(`(?s)waiting for dual-control approval.*00000000-0000-4000-9000-000000000001`),
			},
		},
	})
	if fake.pendingApprovals != 1 {
		t.Fatalf("pending approvals requested = %d, want 1", fake.pendingApprovals)
	}
}

const testExpiresAtConfig = `
resource "keel_api_key" "test" {
  name               = "tf-e2e-exp"
  scope              = "client"
  expires_at         = "2027-01-01T00:00:00+00:00"
  agent_principal_id = "5b1e0f4c-3a57-4d1e-9f43-2c8e6a9d7b10"
}
`

// TestAPIKeyExpiresAtComparedByInstant: Keel returns expires_at re-serialized
// ("+00:00" becomes "Z"). Apply must keep the configured spelling instead of
// reporting an inconsistent result, and the post-apply plan must be empty.
func TestAPIKeyExpiresAtComparedByInstant(t *testing.T) {
	requireTerraformCLI(t)
	fake := newFakeKeel(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             fake.checkCreatedKeysRevoked,
		Steps: []resource.TestStep{
			{
				Config: fake.providerConfig() + testExpiresAtConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("keel_api_key.test", "expires_at", "2027-01-01T00:00:00+00:00"),
					resource.TestCheckResourceAttr("keel_api_key.test", "agent_principal_id", "5b1e0f4c-3a57-4d1e-9f43-2c8e6a9d7b10"),
				),
			},
			{
				// Another spelling of the same instant: no change.
				Config: fake.providerConfig() + strings.Replace(testExpiresAtConfig, "2027-01-01T00:00:00+00:00", "2027-01-01T01:00:00+01:00", 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("keel_api_key.test", plancheck.ResourceActionNoop),
					},
				},
			},
			{
				// A different instant replaces the key.
				Config: fake.providerConfig() + strings.Replace(testExpiresAtConfig, "2027-01-01T00:00:00+00:00", "2028-01-01T00:00:00Z", 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("keel_api_key.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.TestCheckResourceAttr("keel_api_key.test", "expires_at", "2028-01-01T00:00:00Z"),
			},
		},
	})
}

// TestAPIKeyImportThenPlanIsEmpty: import stores Keel's spelling of
// expires_at ("Z"); a configuration naming the same instant as "+00:00" must
// plan no change rather than a replacement.
func TestAPIKeyImportThenPlanIsEmpty(t *testing.T) {
	requireTerraformCLI(t)
	fake := newFakeKeel(t)
	keyID := fake.addKey(map[string]any{
		"name":       "tf-e2e-exp",
		"scope":      "client",
		"expires_at": "2027-01-01T00:00:00Z",
	})
	// No agent_principal_id: the common case, where an unknown computed value
	// would otherwise force a replacement.
	config := fake.providerConfig() + `
resource "keel_api_key" "test" {
  name       = "tf-e2e-exp"
  scope      = "client"
  expires_at = "2027-01-01T00:00:00+00:00"
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             fake.checkCreatedKeysRevoked,
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       "keel_api_key.test",
				ImportState:        true,
				ImportStateId:      keyID,
				ImportStatePersist: true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if got := states[0].Attributes["expires_at"]; got != "2027-01-01T00:00:00Z" {
						return fmt.Errorf("imported expires_at = %q, want Keel's spelling", got)
					}
					if got := states[0].Attributes["project_id"]; got != fakeProjectID {
						return fmt.Errorf("imported project_id = %q, want %q", got, fakeProjectID)
					}
					return nil
				},
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// TestAPIKeyScopeValidatedAtPlan: an unknown scope fails before any request.
func TestAPIKeyScopeValidatedAtPlan(t *testing.T) {
	requireTerraformCLI(t)
	fake := newFakeKeel(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fake.providerConfig() + `
resource "keel_api_key" "test" {
  scope = "Admin"
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`value must be one of: "admin", "client", "approval"`),
			},
		},
	})
}
