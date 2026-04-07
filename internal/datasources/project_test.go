package datasources_test

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

func TestAccProjectDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "keel_project" "test" {
  name = "datasource-test-project"
}

data "keel_project" "test" {
  id = keel_project.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.keel_project.test", "name", "datasource-test-project"),
				),
			},
		},
	})
}
