package resources_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccOrganizationMemberResource_basic(t *testing.T) {
	resourceName := "keel_organization_member.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccOrganizationMemberPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationMemberConfig("member"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "org_id", os.Getenv("KEEL_TEST_ORG_ID")),
					resource.TestCheckResourceAttr(resourceName, "user_id", os.Getenv("KEEL_TEST_USER_ID")),
					resource.TestCheckResourceAttr(resourceName, "role", "member"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
				),
			},
			{
				Config: testAccOrganizationMemberConfig("viewer"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "role", "viewer"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccOrganizationMemberImportID(resourceName),
			},
		},
	})
}

func testAccOrganizationMemberConfig(role string) string {
	return fmt.Sprintf(`
resource "keel_organization_member" "test" {
  org_id  = %[1]q
  user_id = %[2]q
  role    = %[3]q
}
`, os.Getenv("KEEL_TEST_ORG_ID"), os.Getenv("KEEL_TEST_USER_ID"), role)
}

func testAccOrganizationMemberImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", resourceName)
		}
		orgID := rs.Primary.Attributes["org_id"]
		userID := rs.Primary.Attributes["user_id"]
		if orgID == "" || userID == "" {
			return "", fmt.Errorf("org_id and user_id must be set")
		}
		return fmt.Sprintf("%s/%s", orgID, userID), nil
	}
}
