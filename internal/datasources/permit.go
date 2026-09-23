package datasources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/client"
	"github.com/keelapi/terraform-provider-keel/internal/validators"
)

// deprecatedDecisionAliases maps filter values Keel no longer accepts to the
// ones it does: Keel reports review decisions as "review", not "challenge".
var deprecatedDecisionAliases = map[string]string{"challenge": "review"}

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
	OutcomeKind   types.String `tfsdk:"outcome_kind"`
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
		Description: "Query Keel permits, newest first (one page of up to 200). Scoped to the project of the provider's API key, which may have admin or client scope.",
		Attributes: map[string]schema.Attribute{
			"decision": schema.StringAttribute{
				Optional:    true,
				Description: "Filter by decision: \"allow\", \"deny\", \"review\", or \"throttle\". \"challenge\" is a deprecated alias for \"review\".",
				Validators: []validator.String{
					validators.OneOfWithDeprecatedAliases([]string{"allow", "deny", "review", "throttle"}, deprecatedDecisionAliases),
				},
			},
			"limit": schema.Int64Attribute{
				Optional:    true,
				Description: "Maximum number of permits to return, from 1 to 200. Keel returns 50 when unset.",
				Validators: []validator.Int64{
					validators.Int64Between(1, 200),
				},
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
							Description: "Permit decision: \"allow\", \"deny\", \"review\", or \"throttle\".",
						},
						"reason": schema.StringAttribute{
							Computed:    true,
							Description: "Reason Keel recorded with the decision; usually a reason code.",
						},
						"reason_code": schema.StringAttribute{
							Computed:    true,
							Description: "Dot-namespaced reason code, from the permit's decision_details.code, or its reason when it has no decision details. Example: budget.daily_cap_exceeded.",
						},
						"outcome_kind": schema.StringAttribute{
							Computed:    true,
							Description: "What the reason code means: \"decision\" (a policy or budget rule decided), \"precondition\" (Keel could not evaluate the request as configured, so no rule decided), or \"unclassified\".",
						},
						"reason_detail": schema.StringAttribute{
							Computed:    true,
							Description: "The permit's decision_details object (decision, code, reason and any additional details) as a JSON string, if present.",
						},
						"outcome_detail": schema.StringAttribute{
							Computed:           true,
							Description:        "Always null: Keel's permit list returns no outcome detail.",
							DeprecationMessage: "Keel's permit list returns no outcome detail, so outcome_detail is always null. It will be removed in the next major version; use reason_detail.",
						},
						"message": schema.StringAttribute{
							Computed:    true,
							Description: "Human-readable reason, from the permit's decision_details.reason, if present.",
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
	data, ok := req.ProviderData.(*client.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected DataSource Configure Type",
			fmt.Sprintf("Expected *client.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	if data.APIKey == nil {
		resp.Diagnostics.AddError(
			"Missing Keel API key",
			"keel_permit reads Keel's /v1/permits route, which requires a Keel API key (admin or client scope). "+
				"Set api_key in the provider configuration or the KEEL_API_KEY environment variable.",
		)
		return
	}
	d.client = data.APIKey
}

func (d *permitDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config permitDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	if !config.Decision.IsNull() && !config.Decision.IsUnknown() {
		decision := config.Decision.ValueString()
		if replacement, ok := deprecatedDecisionAliases[decision]; ok {
			decision = replacement // the validator warns about the alias
		}
		params.Set("decision", decision)
	}
	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
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

	// PermitAuditListResponse. Only the fields the data source exposes are
	// decoded; reason codes and messages live in decision_details.
	var apiResp struct {
		Items []struct {
			ID              string          `json:"id"`
			Decision        string          `json:"decision"`
			Reason          string          `json:"reason"`
			OutcomeKind     string          `json:"outcome_kind"`
			DecisionDetails json.RawMessage `json:"decision_details"`
			CreatedAt       string          `json:"created_at"`
		} `json:"items"`
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	config.Permits = make([]permitModel, len(apiResp.Items))
	for i, p := range apiResp.Items {
		var details struct {
			Code   string `json:"code"`
			Reason string `json:"reason"`
		}
		if len(p.DecisionDetails) > 0 && string(p.DecisionDetails) != "null" {
			if err := json.Unmarshal(p.DecisionDetails, &details); err != nil {
				resp.Diagnostics.AddError("Error parsing response", fmt.Sprintf("permit %s decision_details: %s", p.ID, err))
				return
			}
		}
		reasonCode := details.Code
		if reasonCode == "" {
			reasonCode = p.Reason
		}
		config.Permits[i] = permitModel{
			ID:            types.StringValue(p.ID),
			Decision:      types.StringValue(p.Decision),
			Reason:        types.StringValue(p.Reason),
			ReasonCode:    stringOrNull(reasonCode),
			OutcomeKind:   stringOrNull(p.OutcomeKind),
			ReasonDetail:  rawJSONOrNull(p.DecisionDetails),
			OutcomeDetail: types.StringNull(),
			Message:       stringOrNull(details.Reason),
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

// rawJSONOrNull returns compact JSON, or null for an absent or null value.
func rawJSONOrNull(raw json.RawMessage) types.String {
	if len(raw) == 0 || string(raw) == "null" {
		return types.StringNull()
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return types.StringValue(string(raw))
	}
	return types.StringValue(compact.String())
}
