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

var _ resource.Resource = &budgetEnvelopeResource{}

type budgetEnvelopeResource struct {
	client *client.Client
}

type budgetEnvelopeResourceModel struct {
	ID           types.String  `tfsdk:"id"`
	ProjectID    types.String  `tfsdk:"project_id"`
	Name         types.String  `tfsdk:"name"`
	AmountUSD    types.Float64 `tfsdk:"amount_usd"`
	Period       types.String  `tfsdk:"period"`
	AlertAtPct   types.List    `tfsdk:"alert_at_pct"`
	HardCap      types.Bool    `tfsdk:"hard_cap"`
	SpentUSD     types.Float64 `tfsdk:"spent_usd"`
	RemainingUSD types.Float64 `tfsdk:"remaining_usd"`
}

func NewBudgetEnvelopeResource() resource.Resource {
	return &budgetEnvelopeResource{}
}

func (r *budgetEnvelopeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_budget_envelope"
}

func (r *budgetEnvelopeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Keel budget envelope via the control-plane API. Deletion is not supported by the API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Budget envelope ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:    true,
				Description: "Project ID.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Budget envelope name.",
			},
			"amount_usd": schema.Float64Attribute{
				Required:    true,
				Description: "Budget amount in USD.",
			},
			"period": schema.StringAttribute{
				Required:    true,
				Description: "Budget period: \"daily\", \"weekly\", \"monthly\", or \"quarterly\".",
			},
			"alert_at_pct": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "Alert threshold percentages.",
			},
			"hard_cap": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to enforce a hard spending cap.",
			},
			"spent_usd": schema.Float64Attribute{
				Computed:    true,
				Description: "Amount spent in current period.",
			},
			"remaining_usd": schema.Float64Attribute{
				Computed:    true,
				Description: "Amount remaining in current period.",
			},
		},
	}
}

func (r *budgetEnvelopeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type budgetEnvelopeAPIModel struct {
	ID           string  `json:"id,omitempty"`
	ProjectID    string  `json:"project_id,omitempty"`
	Name         string  `json:"name"`
	AmountUSD    float64 `json:"amount_usd"`
	Period       string  `json:"period"`
	AlertAtPct   []int64 `json:"alert_at_pct,omitempty"`
	HardCap      bool    `json:"hard_cap"`
	SpentUSD     float64 `json:"spent_usd,omitempty"`
	RemainingUSD float64 `json:"remaining_usd,omitempty"`
}

func (r *budgetEnvelopeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan budgetEnvelopeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := budgetEnvelopeAPIModel{
		ProjectID: plan.ProjectID.ValueString(),
		Name:      plan.Name.ValueString(),
		AmountUSD: plan.AmountUSD.ValueFloat64(),
		Period:    plan.Period.ValueString(),
		HardCap:   plan.HardCap.ValueBool(),
	}

	if !plan.AlertAtPct.IsNull() {
		var pcts []int64
		resp.Diagnostics.Append(plan.AlertAtPct.ElementsAs(ctx, &pcts, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.AlertAtPct = pcts
	}

	body, err := r.client.Post(ctx, "/v1/control-plane/budget-envelopes", apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating budget envelope", err.Error())
		return
	}

	var apiResp budgetEnvelopeAPIModel
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = types.StringValue(apiResp.ID)
	plan.SpentUSD = types.Float64Value(apiResp.SpentUSD)
	plan.RemainingUSD = types.Float64Value(apiResp.RemainingUSD)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *budgetEnvelopeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state budgetEnvelopeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use list endpoint and filter by ID.
	body, err := r.client.Get(ctx, "/v1/control-plane/budget-envelopes")
	if err != nil {
		resp.Diagnostics.AddError("Error reading budget envelopes", err.Error())
		return
	}

	var envelopes []budgetEnvelopeAPIModel
	if err := json.Unmarshal(body, &envelopes); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var found *budgetEnvelopeAPIModel
	for _, e := range envelopes {
		if e.ID == state.ID.ValueString() {
			found = &e
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(found.Name)
	state.AmountUSD = types.Float64Value(found.AmountUSD)
	state.Period = types.StringValue(found.Period)
	state.HardCap = types.BoolValue(found.HardCap)
	state.SpentUSD = types.Float64Value(found.SpentUSD)
	state.RemainingUSD = types.Float64Value(found.RemainingUSD)

	if len(found.AlertAtPct) > 0 {
		listVal, diags := types.ListValueFrom(ctx, types.Int64Type, found.AlertAtPct)
		resp.Diagnostics.Append(diags...)
		state.AlertAtPct = listVal
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *budgetEnvelopeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan budgetEnvelopeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state budgetEnvelopeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := budgetEnvelopeAPIModel{
		Name:      plan.Name.ValueString(),
		AmountUSD: plan.AmountUSD.ValueFloat64(),
		Period:    plan.Period.ValueString(),
		HardCap:   plan.HardCap.ValueBool(),
	}

	if !plan.AlertAtPct.IsNull() {
		var pcts []int64
		resp.Diagnostics.Append(plan.AlertAtPct.ElementsAs(ctx, &pcts, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.AlertAtPct = pcts
	}

	body, err := r.client.Patch(ctx, fmt.Sprintf("/v1/control-plane/budget-envelopes/%s", state.ID.ValueString()), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating budget envelope", err.Error())
		return
	}

	var apiResp budgetEnvelopeAPIModel
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = state.ID
	plan.SpentUSD = types.Float64Value(apiResp.SpentUSD)
	plan.RemainingUSD = types.Float64Value(apiResp.RemainingUSD)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete is not supported by the API. We remove from state only and log a warning.
func (r *budgetEnvelopeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Budget envelope not deleted from API",
		"The Keel API does not support deleting budget envelopes. The resource has been removed from Terraform state but still exists in the API.",
	)
}
