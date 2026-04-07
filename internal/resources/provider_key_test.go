package resources_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProviderKeyResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "keel_project" "test" {
  name = "provider-key-test-project"
}

resource "keel_provider_key" "test" {
  project_id = keel_project.test.id
  provider   = "openai"
  key_value  = "sk-test-key-value"
  enabled    = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("keel_provider_key.test", "provider", "openai"),
					resource.TestCheckResourceAttr("keel_provider_key.test", "enabled", "true"),
				),
			},
		},
	})
}
