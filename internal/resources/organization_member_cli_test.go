package resources_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const fakeMemberUserID = "9605e229-53ce-4b24-9c01-e6e2057d202f"

func memberConfig(fake *fakeKeel, role string) string {
	return fake.userTokenProviderConfig() + fmt.Sprintf(`
resource "keel_organization_member" "test" {
  org_id  = %q
  user_id = %q
  role    = %q
}
`, fakeOrgID, fakeMemberUserID, role)
}

// TestOrganizationMemberLifecycleWithUserToken: create, role update, import
// and destroy with only a user token configured.
func TestOrganizationMemberLifecycleWithUserToken(t *testing.T) {
	requireTerraformCLI(t)
	fake := newFakeKeel(t)
	name := "keel_organization_member.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             fake.checkMembersRemoved,
		Steps: []resource.TestStep{
			{
				Config: memberConfig(fake, "member"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(name, "id"),
					resource.TestCheckResourceAttr(name, "role", "member"),
				),
			},
			{
				Config: memberConfig(fake, "viewer"),
				Check:  resource.TestCheckResourceAttr(name, "role", "viewer"),
			},
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     fakeOrgID + "/" + fakeMemberUserID,
			},
		},
	})
}

// TestOrganizationMemberRoleValidatedAtPlan: Keel stores roles lowercase, so
// "Member" would apply as "member" and fail with an inconsistent result.
func TestOrganizationMemberRoleValidatedAtPlan(t *testing.T) {
	requireTerraformCLI(t)
	fake := newFakeKeel(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      memberConfig(fake, "Member"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`must\s+be\s+one\s+of:\s+"owner",\s+"admin",\s+"member",\s+"viewer",\s+got:\s+"Member"`),
			},
		},
	})
}

// TestOrganizationMemberWithoutUserToken: an API key alone cannot manage
// membership; the provider says so at plan time instead of sending it.
func TestOrganizationMemberWithoutUserToken(t *testing.T) {
	requireTerraformCLI(t)
	fake := newFakeKeel(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fake.providerConfig() + fmt.Sprintf(`
resource "keel_organization_member" "test" {
  org_id  = %q
  user_id = %q
  role    = "member"
}
`, fakeOrgID, fakeMemberUserID),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`API\s+keys\s+cannot\s+manage\s+organization\s+membership`),
			},
		},
	})
}
