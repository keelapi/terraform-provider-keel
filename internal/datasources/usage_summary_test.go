package datasources_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUsageSummaryDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "keel_project" "test" {
  name = "usage-datasource-test"
}

data "keel_usage_summary" "test" {
  project_id = keel_project.test.id
  from       = "2026-04-01"
  to         = "2026-04-30"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.keel_usage_summary.test", "total_cost_usd"),
				),
			},
		},
	})
}
