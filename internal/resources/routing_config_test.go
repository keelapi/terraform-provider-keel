package resources_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRoutingConfigResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "keel_project" "test" {
  name = "routing-test-project"
}

resource "keel_routing_config" "test" {
  project_id = keel_project.test.id

  route {
    provider = "openai"
    model    = "gpt-4o"
    weight   = 70
    priority = 1
  }

  route {
    provider = "anthropic"
    model    = "claude-3-5-sonnet"
    weight   = 30
    priority = 2
  }

  fallback_provider = "openai"
  fallback_model    = "gpt-4o-mini"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("keel_routing_config.test", "id"),
					resource.TestCheckResourceAttr("keel_routing_config.test", "fallback_provider", "openai"),
				),
			},
		},
	})
}
