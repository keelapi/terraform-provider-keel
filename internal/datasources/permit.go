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

var _ datasource.DataSource = &permitDataSource{}

type permitDataSource struct {
	client *client.Client
}

type permitDataSourceModel struct {
	Decision types.String  `tfsdk:"decision"`
	Limit    types.Int64   `tfsdk:"limit"`
	Permits  []permitModel `tfsdk:"permits"`
}

type permitModel struct {
	ID            types.String `tfsdk:"id"`
	Decision      types.String `tfsdk:"decision"`
	Reason        types.String `tfsdk:"reason"`
	ReasonCode    types.String `tfsdk:"reason_code"`
	ReasonDetail  types.String `tfsdk:"reason_detail"`
	OutcomeDetail types.String `tfsdk:"outcome_detail"`
	Message       types.String `tfsdk:"message"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

func NewPermitDataSource() datasource.DataSource {
	return &permitDataSource{}
}

func (d *permitDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_permit"
}

func (d *permitDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Query Keel permits. Scoped by API key, not by project URL.",
		Attributes: map[string]schema.Attribute{
			"decision": schema.StringAttribute{
				Optional:    true,
				Description: "Filter by decision: \"allow\", \"deny\", or \"challenge\".",
			},
			"limit": schema.Int64Attribute{
				Optional:    true,
				Description: "Maximum number of permits to return.",
			},
			"permits": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of permits.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Permit ID.",
						},
						"decision": schema.StringAttribute{
							Computed:    true,
							Description: "Permit decision.",
						},
						"reason": schema.StringAttribute{
							Computed:    true,
							Description: "Permit reason.",
						},
						"reason_code": schema.StringAttribute{
							Computed:    true,
							Description: "Dot-namespaced reason code (Shape D). Example: budget.daily_cap_exceeded.",
						},
						"reason_detail": schema.StringAttribute{
							Computed:    true,
							Description: "Structured reason detail as JSON string.",
						},
						"outcome_detail": schema.StringAttribute{
							Computed:    true,
							Description: "Structured outcome detail as JSON string (e.g. retry_after_seconds).",
						},
						"message": schema.StringAttribute{
							Computed:    true,
							Description: "Human-readable message from the permit decision.",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "Permit timestamp.",
						},
					},
				},
			},
		},
	}
}

func (d *permitDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *permitDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config permitDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	if !config.Decision.IsNull() {
		params.Set("decision", config.Decision.ValueString())
	}
	if !config.Limit.IsNull() {
		params.Set("limit", fmt.Sprintf("%d", config.Limit.ValueInt64()))
	}

	path := "/v1/permits"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	body, err := d.client.Get(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError("Error reading permits", err.Error())
		return
	}

	var apiResp struct {
		Items []struct {
			ID            string          `json:"id"`
			Decision      string          `json:"decision"`
			Reason        string          `json:"reason"`
			ReasonCode    string          `json:"reason_code"`
			ReasonDetail  json.RawMessage `json:"reason_detail"`
			OutcomeDetail json.RawMessage `json:"outcome_detail"`
			Message       string          `json:"message"`
			CreatedAt     string          `json:"created_at"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	config.Permits = make([]permitModel, len(apiResp.Items))
	for i, p := range apiResp.Items {
		config.Permits[i] = permitModel{
			ID:            types.StringValue(p.ID),
			Decision:      types.StringValue(p.Decision),
			Reason:        types.StringValue(p.Reason),
			ReasonCode:    stringOrNull(p.ReasonCode),
			ReasonDetail:  rawJSONOrNull(p.ReasonDetail),
			OutcomeDetail: rawJSONOrNull(p.OutcomeDetail),
			Message:       stringOrNull(p.Message),
			CreatedAt:     types.StringValue(p.CreatedAt),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func rawJSONOrNull(raw json.RawMessage) types.String {
	if len(raw) == 0 || string(raw) == "null" {
		return types.StringNull()
	}
	return types.StringValue(string(raw))
}
