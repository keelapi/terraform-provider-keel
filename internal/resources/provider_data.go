package resources

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

const (
	missingAPIKeySummary = "Missing Keel API key"
	missingAPIKeyDetail  = "keel_api_key creates and revokes keys through Keel's /v1/api-keys routes, which require an admin-scope Keel API key. " +
		"Set api_key in the provider configuration or the KEEL_API_KEY environment variable."

	missingUserTokenSummary = "Missing Keel user token"
	missingUserTokenDetail  = "API keys cannot manage organization membership; set KEEL_USER_TOKEN to a Keel user access token — short-lived.\n\n" +
		"keel_organization_member calls Keel's organization member routes, which accept only a signed-in user's credential. " +
		"Set the provider's user_token argument or the KEEL_USER_TOKEN environment variable to an access token for an owner or admin of the organization. " +
		"A Keel dashboard session token expires after 30 minutes, so supply a fresh token for each run."

	rejectedUserTokenHint = "Keel did not accept the user token. User tokens are short-lived: set KEEL_USER_TOKEN (or user_token) " +
		"to a fresh token for an owner or admin of the organization."
)

// configuredProviderData returns the provider's credentials, or nil while the
// provider is not configured yet (validation-only calls) or on a type error.
func configuredProviderData(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *client.ProviderData {
	if req.ProviderData == nil {
		return nil
	}
	data, ok := req.ProviderData.(*client.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return nil
	}
	return data
}

// userTokenErrorDetail describes a failed organization member request. A 401,
// or the 429 Keel sends in place of repeated 401s, almost always means the
// short-lived user token expired.
func userTokenErrorDetail(err error) string {
	detail := err.Error()
	var apiErr *client.APIError
	var throttled *client.ThrottledError
	if (errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized) ||
		(errors.As(err, &throttled) && throttled.Code == "auth_failure_rate_limited") {
		detail += "\n\n" + rejectedUserTokenHint
	}
	return detail
}
