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

var _ resource.Resource = &providerKeyResource{}

type providerKeyResource struct {
	client *client.Client
}

type providerKeyResourceModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	Provider  types.String `tfsdk:"provider"`
	KeyValue  types.String `tfsdk:"key_value"`
	Enabled   types.Bool   `tfsdk:"enabled"`
}

func NewProviderKeyResource() resource.Resource {
	return &providerKeyResource{}
}

func (r *providerKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_provider_key"
}

func (r *providerKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Keel provider key (BYOK - Mode A). Keyed by provider name, not by ID.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Required:    true,
				Description: "Project ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"provider": schema.StringAttribute{
				Required:    true,
				Description: "AI provider: \"openai\", \"anthropic\", \"google\", \"xai\", or \"meta\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key_value": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Provider API key value. Write-only — never read back from the API.",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the provider key is enabled.",
			},
		},
	}
}

func (r *providerKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type providerKeyAPIRequest struct {
	KeyValue string `json:"key_value"`
	Enabled  bool   `json:"enabled"`
}

type providerListItem struct {
	Provider  string `json:"provider"`
	HasKey    bool   `json:"has_key"`
	Enabled   bool   `json:"enabled"`
}

func (r *providerKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan providerKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := providerKeyAPIRequest{
		KeyValue: plan.KeyValue.ValueString(),
		Enabled:  plan.Enabled.ValueBool(),
	}

	// Create and update use the same PUT endpoint keyed by provider name.
	_, err := r.client.Put(ctx, fmt.Sprintf("/v1/projects/%s/providers/%s/key",
		plan.ProjectID.ValueString(), plan.Provider.ValueString()), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating provider key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *providerKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state providerKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// No single-provider GET — list all providers and find by name.
	body, err := r.client.Get(ctx, fmt.Sprintf("/v1/projects/%s/providers", state.ProjectID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error listing providers", err.Error())
		return
	}

	var providers []providerListItem
	if err := json.Unmarshal(body, &providers); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var found *providerListItem
	for _, p := range providers {
		if p.Provider == state.Provider.ValueString() {
			found = &p
			break
		}
	}

	if found == nil || !found.HasKey {
		// Provider key no longer exists.
		resp.State.RemoveResource(ctx)
		return
	}

	state.Enabled = types.BoolValue(found.Enabled)
	// key_value is write-only — keep state value

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *providerKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan providerKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := providerKeyAPIRequest{
		KeyValue: plan.KeyValue.ValueString(),
		Enabled:  plan.Enabled.ValueBool(),
	}

	// Same PUT endpoint for create and update.
	_, err := r.client.Put(ctx, fmt.Sprintf("/v1/projects/%s/providers/%s/key",
		plan.ProjectID.ValueString(), plan.Provider.ValueString()), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating provider key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *providerKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state providerKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, fmt.Sprintf("/v1/projects/%s/providers/%s/key",
		state.ProjectID.ValueString(), state.Provider.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error deleting provider key", err.Error())
	}
}
