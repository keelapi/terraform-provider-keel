package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

var _ resource.Resource = &policyRuleResource{}

type policyRuleResource struct {
	client *client.Client
}

type policyRuleResourceModel struct {
	ID        types.String         `tfsdk:"id"`
	Name      types.String         `tfsdk:"name"`
	Priority  types.Int64          `tfsdk:"priority"`
	Condition *policyRuleCondition `tfsdk:"condition"`
	Action    types.String         `tfsdk:"action"`
	Reason    types.String         `tfsdk:"reason"`
	Enabled   types.Bool           `tfsdk:"enabled"`
}

type policyRuleCondition struct {
	Field    types.String `tfsdk:"field"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
}

func NewPolicyRuleResource() resource.Resource {
	return &policyRuleResource{}
}

func (r *policyRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_rule"
}

func (r *policyRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Keel policy. Policies are scoped by API key, not by project URL.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Policy ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Policy name.",
			},
			"priority": schema.Int64Attribute{
				Required:    true,
				Description: "Policy priority (lower = higher priority).",
			},
			"action": schema.StringAttribute{
				Required:    true,
				Description: "Action: \"allow\", \"deny\", or \"challenge\".",
			},
			"reason": schema.StringAttribute{
				Optional:    true,
				Description: "Reason shown when policy fires.",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the policy is enabled.",
			},
		},
		Blocks: map[string]schema.Block{
			"condition": schema.SingleNestedBlock{
				Description: "Policy condition.",
				Attributes: map[string]schema.Attribute{
					"field": schema.StringAttribute{
						Required:    true,
						Description: "Field to evaluate.",
					},
					"operator": schema.StringAttribute{
						Required:    true,
						Description: "Comparison operator.",
					},
					"value": schema.StringAttribute{
						Required:    true,
						Description: "Value to compare against.",
					},
				},
			},
		},
	}
}

func (r *policyRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.Client")
		return
	}
	r.client = c
}

type policyRuleAPIModel struct {
	ID        string                  `json:"id,omitempty"`
	Name      string                  `json:"name"`
	Priority  int64                   `json:"priority"`
	Condition *policyRuleConditionAPI `json:"condition,omitempty"`
	Action    string                  `json:"action"`
	Reason    string                  `json:"reason,omitempty"`
	Enabled   bool                    `json:"enabled"`
}

type policyRuleConditionAPI struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

func (r *policyRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := policyRuleAPIModel{
		Name:     plan.Name.ValueString(),
		Priority: plan.Priority.ValueInt64(),
		Action:   plan.Action.ValueString(),
		Reason:   plan.Reason.ValueString(),
		Enabled:  plan.Enabled.ValueBool(),
	}
	if plan.Condition != nil {
		apiReq.Condition = &policyRuleConditionAPI{
			Field:    plan.Condition.Field.ValueString(),
			Operator: plan.Condition.Operator.ValueString(),
			Value:    plan.Condition.Value.ValueString(),
		}
	}

	body, err := r.client.Post(ctx, "/v1/policies", apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating policy", err.Error())
		return
	}

	var apiResp policyRuleAPIModel
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = types.StringValue(apiResp.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *policyRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state policyRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// No single-resource GET — use list endpoint and filter by ID.
	body, err := r.client.Get(ctx, "/v1/policies")
	if err != nil {
		resp.Diagnostics.AddError("Error reading policies", err.Error())
		return
	}

	var policies []policyRuleAPIModel
	if err := json.Unmarshal(body, &policies); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var found *policyRuleAPIModel
	for _, p := range policies {
		if p.ID == state.ID.ValueString() {
			found = &p
			break
		}
	}

	if found == nil {
		// Policy no longer exists.
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(found.Name)
	state.Priority = types.Int64Value(found.Priority)
	state.Action = types.StringValue(found.Action)
	state.Enabled = types.BoolValue(found.Enabled)
	if found.Reason != "" {
		state.Reason = types.StringValue(found.Reason)
	}
	if found.Condition != nil {
		state.Condition = &policyRuleCondition{
			Field:    types.StringValue(found.Condition.Field),
			Operator: types.StringValue(found.Condition.Operator),
			Value:    types.StringValue(found.Condition.Value),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *policyRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan policyRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state policyRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := policyRuleAPIModel{
		Name:     plan.Name.ValueString(),
		Priority: plan.Priority.ValueInt64(),
		Action:   plan.Action.ValueString(),
		Reason:   plan.Reason.ValueString(),
		Enabled:  plan.Enabled.ValueBool(),
	}
	if plan.Condition != nil {
		apiReq.Condition = &policyRuleConditionAPI{
			Field:    plan.Condition.Field.ValueString(),
			Operator: plan.Condition.Operator.ValueString(),
			Value:    plan.Condition.Value.ValueString(),
		}
	}

	_, err := r.client.Patch(ctx, fmt.Sprintf("/v1/policies/%s", state.ID.ValueString()), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating policy", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *policyRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, fmt.Sprintf("/v1/policies/%s", state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error deleting policy", err.Error())
	}
}
