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
	BaseURL types.String `tfsdk:"base_url"`
	APIKey  types.String `tfsdk:"api_key"`
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
		Description: "Manage Keel AI governance resources.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Keel API base URL. Can also be set via KEEL_BASE_URL env var.",
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Keel API key. Can also be set via KEEL_API_KEY env var.",
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

	apiKey := ""
	if !config.APIKey.IsNull() && !config.APIKey.IsUnknown() {
		apiKey = config.APIKey.ValueString()
	} else if v := os.Getenv("KEEL_API_KEY"); v != "" {
		apiKey = v
	}

	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"The Keel API key must be set in the provider configuration or via the KEEL_API_KEY environment variable.",
		)
		return
	}

	c := client.New(baseURL, apiKey)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *keelProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewProjectResource,
		resources.NewAPIKeyResource,
		resources.NewPolicyRuleResource,
		resources.NewBudgetEnvelopeResource,
		resources.NewRoutingConfigResource,
		resources.NewProviderKeyResource,
	}
}

func (p *keelProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewProjectDataSource,
		datasources.NewPermitDataSource,
		datasources.NewUsageSummaryDataSource,
	}
}
