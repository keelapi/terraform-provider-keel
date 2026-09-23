package resources_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/keelapi/terraform-provider-keel/internal/provider"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"keel": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func testAccPreCheck(t *testing.T) {
	if os.Getenv("KEEL_API_KEY") == "" {
		t.Fatal("KEEL_API_KEY must be set for acceptance tests")
	}
}

func testAccOrganizationMemberPreCheck(t *testing.T) {
	// Keel's organization member routes accept a user token, not an API key.
	if os.Getenv("KEEL_USER_TOKEN") == "" {
		t.Fatal("KEEL_USER_TOKEN must be set for organization member acceptance tests")
	}
	if os.Getenv("KEEL_TEST_ORG_ID") == "" {
		t.Fatal("KEEL_TEST_ORG_ID must be set for organization member acceptance tests")
	}
	if os.Getenv("KEEL_TEST_USER_ID") == "" {
		t.Fatal("KEEL_TEST_USER_ID must be set for organization member acceptance tests")
	}
}
