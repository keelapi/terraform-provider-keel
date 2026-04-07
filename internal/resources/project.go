package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

var (
	_ resource.Resource                = &projectResource{}
	_ resource.ResourceWithImportState = &projectResource{}
)

type projectResource struct {
	client *client.Client
}

type projectResourceModel struct {
	ID          types.String   `tfsdk:"id"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	Settings    *projectSettings `tfsdk:"settings"`
	CreatedAt   types.String   `tfsdk:"created_at"`
	UpdatedAt   types.String   `tfsdk:"updated_at"`
}

type projectSettings struct {
	DefaultProvider types.String  `tfsdk:"default_provider"`
	DefaultModel    types.String  `tfsdk:"default_model"`
	BudgetLimitUSD  types.Float64 `tfsdk:"budget_limit_usd"`
	RateLimitRPM    types.Int64   `tfsdk:"rate_limit_rpm"`
}

func NewProjectResource() resource.Resource {
	return &projectResource{}
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Keel project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Project ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Project name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Project description.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Last update timestamp.",
			},
		},
		Blocks: map[string]schema.Block{
			"settings": schema.SingleNestedBlock{
				Description: "Project settings.",
				Attributes: map[string]schema.Attribute{
					"default_provider": schema.StringAttribute{
						Optional:    true,
						Description: "Default AI provider.",
					},
					"default_model": schema.StringAttribute{
						Optional:    true,
						Description: "Default AI model.",
					},
					"budget_limit_usd": schema.Float64Attribute{
						Optional:    true,
						Description: "Budget limit in USD.",
					},
					"rate_limit_rpm": schema.Int64Attribute{
						Optional:    true,
						Description: "Rate limit in requests per minute.",
					},
				},
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type projectAPIModel struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
	CreatedAt   string                 `json:"created_at,omitempty"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := projectAPIModel{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}
	if plan.Settings != nil {
		apiReq.Settings = map[string]interface{}{}
		if !plan.Settings.DefaultProvider.IsNull() {
			apiReq.Settings["default_provider"] = plan.Settings.DefaultProvider.ValueString()
		}
		if !plan.Settings.DefaultModel.IsNull() {
			apiReq.Settings["default_model"] = plan.Settings.DefaultModel.ValueString()
		}
		if !plan.Settings.BudgetLimitUSD.IsNull() {
			apiReq.Settings["budget_limit_usd"] = plan.Settings.BudgetLimitUSD.ValueFloat64()
		}
		if !plan.Settings.RateLimitRPM.IsNull() {
			apiReq.Settings["rate_limit_rpm"] = plan.Settings.RateLimitRPM.ValueInt64()
		}
	}

	body, err := r.client.Post(ctx, "/v1/projects", apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}

	var apiResp projectAPIModel
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = types.StringValue(apiResp.ID)
	plan.CreatedAt = types.StringValue(apiResp.CreatedAt)
	plan.UpdatedAt = types.StringValue(apiResp.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, statusCode, err := r.client.GetWithStatus(ctx, fmt.Sprintf("/v1/projects/%s", state.ID.ValueString()))
	if err != nil {
		if statusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	var apiResp projectAPIModel
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	state.Name = types.StringValue(apiResp.Name)
	if apiResp.Description != "" {
		state.Description = types.StringValue(apiResp.Description)
	}
	state.CreatedAt = types.StringValue(apiResp.CreatedAt)
	state.UpdatedAt = types.StringValue(apiResp.UpdatedAt)

	if apiResp.Settings != nil && state.Settings != nil {
		if v, ok := apiResp.Settings["default_provider"].(string); ok {
			state.Settings.DefaultProvider = types.StringValue(v)
		}
		if v, ok := apiResp.Settings["default_model"].(string); ok {
			state.Settings.DefaultModel = types.StringValue(v)
		}
		if v, ok := apiResp.Settings["budget_limit_usd"].(float64); ok {
			state.Settings.BudgetLimitUSD = types.Float64Value(v)
		}
		if v, ok := apiResp.Settings["rate_limit_rpm"].(float64); ok {
			state.Settings.RateLimitRPM = types.Int64Value(int64(v))
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := projectAPIModel{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}
	if plan.Settings != nil {
		apiReq.Settings = map[string]interface{}{}
		if !plan.Settings.DefaultProvider.IsNull() {
			apiReq.Settings["default_provider"] = plan.Settings.DefaultProvider.ValueString()
		}
		if !plan.Settings.DefaultModel.IsNull() {
			apiReq.Settings["default_model"] = plan.Settings.DefaultModel.ValueString()
		}
		if !plan.Settings.BudgetLimitUSD.IsNull() {
			apiReq.Settings["budget_limit_usd"] = plan.Settings.BudgetLimitUSD.ValueFloat64()
		}
		if !plan.Settings.RateLimitRPM.IsNull() {
			apiReq.Settings["rate_limit_rpm"] = plan.Settings.RateLimitRPM.ValueInt64()
		}
	}

	body, err := r.client.Patch(ctx, fmt.Sprintf("/v1/projects/%s", state.ID.ValueString()), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}

	var apiResp projectAPIModel
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = state.ID
	plan.CreatedAt = state.CreatedAt
	plan.UpdatedAt = types.StringValue(apiResp.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, fmt.Sprintf("/v1/projects/%s", state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error deleting project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
