package datasources_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccPreCheck(t *testing.T) {
	if os.Getenv("KEEL_API_KEY") == "" {
		t.Fatal("KEEL_API_KEY must be set for acceptance tests")
	}
}

func TestAccPermitDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "keel_permit" "test" {
  decision = "deny"
  limit    = 5
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.keel_permit.test", "permits.#"),
				),
			},
		},
	})
}
