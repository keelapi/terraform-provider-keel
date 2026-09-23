package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/timestamp"
)

func TestAPIKeyConfigMatchesState(t *testing.T) {
	state := apiKeyResourceModel{
		Name:             types.StringValue("backend-service"),
		Description:      types.StringNull(),
		Scope:            types.StringValue("client"),
		AgentPrincipalID: types.StringNull(),
		ExpiresAt:        timestamp.NewValue("2027-01-01T00:00:00Z"),
	}
	base := func() (apiKeyResourceModel, apiKeyResourceModel) {
		config := state
		config.ExpiresAt = timestamp.NewValue("2027-01-01T00:00:00+00:00")
		plan := config
		return config, plan
	}

	config, plan := base()
	if !apiKeyConfigMatchesState(config, plan, state) {
		t.Fatal("a re-spelled expires_at must match")
	}

	config, plan = base()
	config.AgentPrincipalID = types.StringUnknown()
	if apiKeyConfigMatchesState(config, plan, state) {
		t.Fatal("an unknown configured agent_principal_id must not match")
	}

	config, plan = base()
	config.ExpiresAt = timestamp.RFC3339{StringValue: types.StringUnknown()}
	if apiKeyConfigMatchesState(config, plan, state) {
		t.Fatal("an unknown configured expires_at must not match")
	}

	config, plan = base()
	config.ExpiresAt = timestamp.NewValue("2027-06-01T00:00:00Z")
	if apiKeyConfigMatchesState(config, plan, state) {
		t.Fatal("a different expires_at must not match")
	}

	config, plan = base()
	config.Name = types.StringValue("renamed")
	if apiKeyConfigMatchesState(config, plan, state) {
		t.Fatal("a different name must not match")
	}

	config, plan = base()
	config.ExpiresAt = timestamp.NewNull()
	if !apiKeyConfigMatchesState(config, plan, state) {
		t.Fatal("an unset expires_at (optional and computed) must match")
	}
}
