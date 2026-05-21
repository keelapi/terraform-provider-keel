package resources_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAPIKeyResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "keel_api_key" "test" {
  name        = "test-key"
  description = "Terraform acceptance test key"
  scope       = "client"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("keel_api_key.test", "id"),
					resource.TestCheckResourceAttr("keel_api_key.test", "name", "test-key"),
					resource.TestCheckResourceAttr("keel_api_key.test", "description", "Terraform acceptance test key"),
					resource.TestCheckResourceAttr("keel_api_key.test", "scope", "client"),
					resource.TestCheckResourceAttrSet("keel_api_key.test", "project_id"),
					resource.TestCheckResourceAttrSet("keel_api_key.test", "prefix"),
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
