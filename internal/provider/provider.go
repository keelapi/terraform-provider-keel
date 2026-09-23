package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/client"
	"github.com/keelapi/terraform-provider-keel/internal/datasources"
	"github.com/keelapi/terraform-provider-keel/internal/resources"
)

var _ provider.Provider = &keelProvider{}

type keelProvider struct {
	version string
}

type keelProviderModel struct {
	BaseURL   types.String `tfsdk:"base_url"`
	APIKey    types.String `tfsdk:"api_key"`
	UserToken types.String `tfsdk:"user_token"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &keelProvider{
			version: version,
		}
	}
}

func (p *keelProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "keel"
	resp.Version = p.version
}

func (p *keelProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Keel API keys and organization membership, and query permits.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Keel API base URL. Can also be set via KEEL_BASE_URL env var.",
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Keel API key, used by keel_api_key (admin scope required) and keel_permit (admin or client scope). Can also be set via KEEL_API_KEY env var.",
			},
			"user_token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Keel user access token, used only by keel_organization_member: Keel's organization member routes accept a signed-in user's credential, not an API key. The user must be an owner or admin of the organization. User tokens are short-lived (a Keel dashboard session token expires after 30 minutes), so supply a fresh one for each run, typically through the KEEL_USER_TOKEN env var.",
			},
		},
	}
}

func (p *keelProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config keelProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := "https://api.keelapi.com"
	if !config.BaseURL.IsNull() && !config.BaseURL.IsUnknown() {
		baseURL = config.BaseURL.ValueString()
	} else if v := os.Getenv("KEEL_BASE_URL"); v != "" {
		baseURL = v
	}

	apiKey := stringFromConfigOrEnv(config.APIKey, "KEEL_API_KEY")
	userToken := stringFromConfigOrEnv(config.UserToken, "KEEL_USER_TOKEN")

	if apiKey == "" && userToken == "" {
		resp.Diagnostics.AddError(
			"Missing Keel credentials",
			"Set api_key (or the KEEL_API_KEY environment variable) to a Keel API key for keel_api_key and keel_permit, "+
				"and user_token (or KEEL_USER_TOKEN) to a Keel user access token for keel_organization_member.",
		)
		return
	}

	data := &client.ProviderData{}
	if apiKey != "" {
		data.APIKey = client.New(baseURL, apiKey)
	}
	if userToken != "" {
		data.UserToken = client.New(baseURL, userToken)
	}
	resp.DataSourceData = data
	resp.ResourceData = data
}

// stringFromConfigOrEnv returns the configured value when it is known, else
// the environment variable.
func stringFromConfigOrEnv(value types.String, envVar string) string {
	if !value.IsNull() && !value.IsUnknown() {
		return value.ValueString()
	}
	return os.Getenv(envVar)
}

func (p *keelProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewAPIKeyResource,
		resources.NewOrganizationMemberResource,
	}
}

func (p *keelProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewPermitDataSource,
	}
}
