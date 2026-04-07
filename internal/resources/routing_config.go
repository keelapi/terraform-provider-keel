package resources

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

var _ resource.Resource = &routingConfigResource{}

type routingConfigResource struct {
	client *client.Client
}

type routingConfigResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ProjectID        types.String `tfsdk:"project_id"`
	Routes           []routeModel `tfsdk:"route"`
	FallbackProvider types.String `tfsdk:"fallback_provider"`
	FallbackModel    types.String `tfsdk:"fallback_model"`
}

type routeModel struct {
	Provider types.String `tfsdk:"provider"`
	Model    types.String `tfsdk:"model"`
	Weight   types.Int64  `tfsdk:"weight"`
	Priority types.Int64  `tfsdk:"priority"`
}

func NewRoutingConfigResource() resource.Resource {
	return &routingConfigResource{}
}

func (r *routingConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_routing_config"
}

func (r *routingConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Keel routing configuration via the control-plane API. Any change forces replacement because update/delete are not supported.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Routing policy ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:    true,
				Description: "Project ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"fallback_provider": schema.StringAttribute{
				Optional:    true,
				Description: "Fallback provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"fallback_model": schema.StringAttribute{
				Optional:    true,
				Description: "Fallback model.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"route": schema.ListNestedBlock{
				Description: "Route definitions. Changes force replacement.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"provider": schema.StringAttribute{
							Required:    true,
							Description: "AI provider name.",
						},
						"model": schema.StringAttribute{
							Required:    true,
							Description: "Model name.",
						},
						"weight": schema.Int64Attribute{
							Required:    true,
							Description: "Traffic weight percentage.",
						},
						"priority": schema.Int64Attribute{
							Required:    true,
							Description: "Route priority.",
						},
					},
				},
			},
		},
	}
}

func (r *routingConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type routingConfigAPIModel struct {
	ID               string          `json:"id,omitempty"`
	ProjectID        string          `json:"project_id,omitempty"`
	Routes           []routeAPIModel `json:"routes,omitempty"`
	FallbackProvider string          `json:"fallback_provider,omitempty"`
	FallbackModel    string          `json:"fallback_model,omitempty"`
}

type routeAPIModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Weight   int64  `json:"weight"`
	Priority int64  `json:"priority"`
}

func (r *routingConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan routingConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := routingConfigAPIModel{
		ProjectID:        plan.ProjectID.ValueString(),
		FallbackProvider: plan.FallbackProvider.ValueString(),
		FallbackModel:    plan.FallbackModel.ValueString(),
	}
	for _, route := range plan.Routes {
		apiReq.Routes = append(apiReq.Routes, routeAPIModel{
			Provider: route.Provider.ValueString(),
			Model:    route.Model.ValueString(),
			Weight:   route.Weight.ValueInt64(),
			Priority: route.Priority.ValueInt64(),
		})
	}

	body, err := r.client.Post(ctx, "/v1/control-plane/routing-policies", apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating routing policy", err.Error())
		return
	}

	var apiResp routingConfigAPIModel
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = types.StringValue(apiResp.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *routingConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state routingConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use list endpoint and filter by ID.
	body, err := r.client.Get(ctx, "/v1/control-plane/routing-policies")
	if err != nil {
		resp.Diagnostics.AddError("Error reading routing policies", err.Error())
		return
	}

	var policies []routingConfigAPIModel
	if err := json.Unmarshal(body, &policies); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var found *routingConfigAPIModel
	for _, p := range policies {
		if p.ID == state.ID.ValueString() {
			found = &p
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Routes = nil
	for _, route := range found.Routes {
		state.Routes = append(state.Routes, routeModel{
			Provider: types.StringValue(route.Provider),
			Model:    types.StringValue(route.Model),
			Weight:   types.Int64Value(route.Weight),
			Priority: types.Int64Value(route.Priority),
		})
	}
	if found.FallbackProvider != "" {
		state.FallbackProvider = types.StringValue(found.FallbackProvider)
	}
	if found.FallbackModel != "" {
		state.FallbackModel = types.StringValue(found.FallbackModel)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update is not supported — all fields use RequiresReplace.
func (r *routingConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Routing policies cannot be updated. Any change requires replacement (destroy + create).",
	)
}

// Delete is not supported by the API. Remove from state only.
func (r *routingConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Routing policy not deleted from API",
		"The Keel API does not support deleting routing policies. The resource has been removed from Terraform state but still exists in the API.",
	)
}
