package resources_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/keelapi/terraform-provider-keel/internal/provider"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"keel": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func TestAccProjectResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "keel_project" "test" {
  name        = "test-project"
  description = "Test project"

  settings {
    default_provider = "openai"
    default_model    = "gpt-4o"
    budget_limit_usd = 500.00
    rate_limit_rpm   = 300
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("keel_project.test", "id"),
					resource.TestCheckResourceAttr("keel_project.test", "name", "test-project"),
					resource.TestCheckResourceAttr("keel_project.test", "description", "Test project"),
				),
			},
			{
				Config: `
resource "keel_project" "test" {
  name        = "test-project-updated"
  description = "Updated test project"

  settings {
    default_provider = "anthropic"
    default_model    = "claude-3-5-sonnet"
    budget_limit_usd = 1000.00
    rate_limit_rpm   = 600
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("keel_project.test", "name", "test-project-updated"),
				),
			},
			{
				ResourceName:      "keel_project.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
