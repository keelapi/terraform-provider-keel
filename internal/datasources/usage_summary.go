package datasources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

var _ datasource.DataSource = &usageSummaryDataSource{}

type usageSummaryDataSource struct {
	client *client.Client
}

type usageSummaryDataSourceModel struct {
	ProjectID     types.String  `tfsdk:"project_id"`
	From          types.String  `tfsdk:"from"`
	To            types.String  `tfsdk:"to"`
	TotalCostUSD  types.Float64 `tfsdk:"total_cost_usd"`
	TotalRequests types.Int64   `tfsdk:"total_requests"`
	TotalTokens   types.Int64   `tfsdk:"total_tokens"`
}

func NewUsageSummaryDataSource() datasource.DataSource {
	return &usageSummaryDataSource{}
}

func (d *usageSummaryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_usage_summary"
}

func (d *usageSummaryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Read Keel usage summary for a project.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Required:    true,
				Description: "Project ID.",
			},
			"from": schema.StringAttribute{
				Required:    true,
				Description: "Start date (YYYY-MM-DD).",
			},
			"to": schema.StringAttribute{
				Required:    true,
				Description: "End date (YYYY-MM-DD).",
			},
			"total_cost_usd": schema.Float64Attribute{
				Computed:    true,
				Description: "Total cost in USD.",
			},
			"total_requests": schema.Int64Attribute{
				Computed:    true,
				Description: "Total number of requests.",
			},
			"total_tokens": schema.Int64Attribute{
				Computed:    true,
				Description: "Total tokens consumed.",
			},
		},
	}
}

func (d *usageSummaryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected DataSource Configure Type", "Expected *client.Client")
		return
	}
	d.client = c
}

func (d *usageSummaryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config usageSummaryDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("from", config.From.ValueString())
	params.Set("to", config.To.ValueString())

	path := fmt.Sprintf("/v1/dashboard/projects/%s/usage/summary?%s", config.ProjectID.ValueString(), params.Encode())

	body, err := d.client.Get(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError("Error reading usage summary", err.Error())
		return
	}

	var apiResp struct {
		TotalCostUSD  float64 `json:"total_cost_usd"`
		TotalRequests int64   `json:"total_requests"`
		TotalTokens   int64   `json:"total_tokens"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	config.TotalCostUSD = types.Float64Value(apiResp.TotalCostUSD)
	config.TotalRequests = types.Int64Value(apiResp.TotalRequests)
	config.TotalTokens = types.Int64Value(apiResp.TotalTokens)

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
