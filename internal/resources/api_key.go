package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

var _ resource.Resource = &apiKeyResource{}

type apiKeyResource struct {
	client *client.Client
}

type apiKeyResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Scope     types.String `tfsdk:"scope"`
	Prefix    types.String `tfsdk:"prefix"`
	RawKey    types.String `tfsdk:"raw_key"`
	CreatedAt types.String `tfsdk:"created_at"`
	Status    types.String `tfsdk:"status"`
}

func NewAPIKeyResource() resource.Resource {
	return &apiKeyResource{}
}

func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Keel API key. API keys are immutable after creation — any change forces replacement. Deletion revokes the key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "API key ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:    true,
				Description: "Project ID this key belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "API key name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"scope": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("project"),
				Description: "Key scope. Default: \"project\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"prefix": schema.StringAttribute{
				Computed:    true,
				Description: "Key prefix (visible portion).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"raw_key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The raw API key. Only available at creation time.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Key status.",
			},
		},
	}
}

func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type apiKeyAPIModel struct {
	ID        string `json:"id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	RawKey    string `json:"raw_key,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	Status    string `json:"status,omitempty"`
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := apiKeyAPIModel{
		Name:  plan.Name.ValueString(),
		Scope: plan.Scope.ValueString(),
	}

	body, err := r.client.Post(ctx, fmt.Sprintf("/v1/projects/%s/api-keys", plan.ProjectID.ValueString()), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating API key", err.Error())
		return
	}

	var apiResp apiKeyAPIModel
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = types.StringValue(apiResp.ID)
	plan.Prefix = types.StringValue(apiResp.Prefix)
	plan.RawKey = types.StringValue(apiResp.RawKey)
	plan.CreatedAt = types.StringValue(apiResp.CreatedAt)
	plan.Status = types.StringValue(apiResp.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// No single-key GET endpoint — use the list endpoint and filter by ID.
	body, err := r.client.Get(ctx, fmt.Sprintf("/v1/projects/%s/api-keys", state.ProjectID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error listing API keys", err.Error())
		return
	}

	var keys []apiKeyAPIModel
	if err := json.Unmarshal(body, &keys); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var found *apiKeyAPIModel
	for _, k := range keys {
		if k.ID == state.ID.ValueString() {
			found = &k
			break
		}
	}

	if found == nil {
		// Key no longer exists (revoked or deleted).
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(found.Name)
	state.Scope = types.StringValue(found.Scope)
	state.Prefix = types.StringValue(found.Prefix)
	state.Status = types.StringValue(found.Status)
	// raw_key is NOT returned on list — keep state value

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update is not supported — all mutable fields use RequiresReplace.
func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"API keys cannot be updated. Any change requires replacement (destroy + create).",
	)
}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// API uses POST revoke, not DELETE.
	_, err := r.client.Post(ctx, fmt.Sprintf("/v1/projects/%s/api-keys/%s/revoke", state.ProjectID.ValueString(), state.ID.ValueString()), nil)
	if err != nil {
		resp.Diagnostics.AddError("Error revoking API key", err.Error())
	}
}
