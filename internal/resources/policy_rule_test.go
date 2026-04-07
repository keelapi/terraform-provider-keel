package resources_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPolicyResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "keel_policy_rule" "test" {
  name       = "test-rule"
  priority   = 10

  condition {
    field    = "resource.attributes.estimated_cost_usd"
    operator = "greater_than"
    value    = "50.00"
  }

  action  = "deny"
  reason  = "Cost too high"
  enabled = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("keel_policy_rule.test", "id"),
					resource.TestCheckResourceAttr("keel_policy_rule.test", "name", "test-rule"),
					resource.TestCheckResourceAttr("keel_policy_rule.test", "action", "deny"),
				),
			},
		},
	})
}
