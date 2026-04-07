package resources_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAPIKeyResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "keel_project" "test" {
  name = "api-key-test-project"
}

resource "keel_api_key" "test" {
  project_id = keel_project.test.id
  name       = "test-key"
  scope      = "project"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("keel_api_key.test", "id"),
					resource.TestCheckResourceAttr("keel_api_key.test", "name", "test-key"),
					resource.TestCheckResourceAttr("keel_api_key.test", "scope", "project"),
					resource.TestCheckResourceAttrSet("keel_api_key.test", "prefix"),
					resource.TestCheckResourceAttrSet("keel_api_key.test", "raw_key"),
				),
			},
		},
	})
}
