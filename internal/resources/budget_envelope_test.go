package resources_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccBudgetEnvelopeResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "keel_project" "test" {
  name = "budget-test-project"
}

resource "keel_budget_envelope" "test" {
  project_id   = keel_project.test.id
  name         = "test-budget"
  amount_usd   = 5000.00
  period       = "monthly"
  alert_at_pct = [50, 75, 90]
  hard_cap     = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("keel_budget_envelope.test", "id"),
					resource.TestCheckResourceAttr("keel_budget_envelope.test", "name", "test-budget"),
					resource.TestCheckResourceAttr("keel_budget_envelope.test", "period", "monthly"),
					resource.TestCheckResourceAttr("keel_budget_envelope.test", "hard_cap", "true"),
				),
			},
		},
	})
}
