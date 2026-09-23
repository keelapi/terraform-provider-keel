package resources_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
