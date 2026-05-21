package resources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

var _ resource.Resource = &apiKeyResource{}
var _ resource.ResourceWithImportState = &apiKeyResource{}

type apiKeyResource struct {
	client *client.Client
}

type apiKeyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Scope       types.String `tfsdk:"scope"`
	CreatedBy   types.String `tfsdk:"created_by"`
	Prefix      types.String `tfsdk:"prefix"`
	RawKey      types.String `tfsdk:"raw_key"`
	CreatedAt   types.String `tfsdk:"created_at"`
	RevokedAt   types.String `tfsdk:"revoked_at"`
	LastUsedAt  types.String `tfsdk:"last_used_at"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	Status      types.String `tfsdk:"status"`
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
				Optional:    true,
				Computed:    true,
				Description: "Project ID this key belongs to. When omitted, the provider uses /v1/api-keys and the Keel API derives the project from the provider API key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "API key description.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"scope": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("admin"),
				Description: "Key scope: \"admin\", \"client\", or \"approval\". Default: \"admin\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_by": schema.StringAttribute{
				Computed:    true,
				Description: "User ID that created the API key, if available.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
			"revoked_at": schema.StringAttribute{
				Computed:    true,
				Description: "Revocation timestamp, if the key has been revoked.",
			},
			"last_used_at": schema.StringAttribute{
				Computed:    true,
				Description: "Last-used timestamp, if the key has been used.",
			},
			"expires_at": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Expiration timestamp, if configured.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Derived key status: \"active\" or \"revoked\".",
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
	ID          string `json:"id,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Scope       string `json:"scope,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
	Prefix      string `json:"prefix,omitempty"`
	RawKey      string `json:"raw_key,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	RevokedAt   string `json:"revoked_at,omitempty"`
	LastUsedAt  string `json:"last_used_at,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

type apiKeyCreateRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Scope       string `json:"scope,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

type apiKeyListResponse struct {
	Items      []apiKeyAPIModel `json:"items"`
	NextCursor string           `json:"next_cursor"`
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scope := plan.Scope.ValueString()
	if !validAPIKeyScope(scope) {
		resp.Diagnostics.AddAttributeError(
			path.Root("scope"),
			"Invalid API Key Scope",
			"Scope must be one of: admin, client, approval.",
		)
		return
	}

	projectID := ""
	if !plan.ProjectID.IsNull() && !plan.ProjectID.IsUnknown() {
		projectID = plan.ProjectID.ValueString()
	}
	if projectID != "" && scope == "approval" {
		resp.Diagnostics.AddAttributeError(
			path.Root("scope"),
			"Invalid Project API Key Scope",
			"Project-scoped API keys support admin and client scopes. Omit project_id to use /v1/api-keys with approval scope.",
		)
		return
	}

	apiReq := apiKeyCreateRequest{
		Scope: scope,
	}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		apiReq.Name = plan.Name.ValueString()
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		apiReq.Description = plan.Description.ValueString()
	}
	if !plan.ExpiresAt.IsNull() && !plan.ExpiresAt.IsUnknown() {
		apiReq.ExpiresAt = plan.ExpiresAt.ValueString()
	}

	body, err := r.client.Post(ctx, apiKeyCollectionPath(projectID), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating API key", err.Error())
		return
	}

	var apiResp apiKeyAPIModel
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	applyAPIKeyToState(&plan, apiResp, true)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := ""
	if !state.ProjectID.IsNull() && !state.ProjectID.IsUnknown() {
		projectID = state.ProjectID.ValueString()
	}

	// No single-key GET endpoint — use the list endpoint and filter by ID.
	found, err := r.findAPIKey(ctx, state.ID.ValueString(), projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing API keys", err.Error())
		return
	}

	if found == nil || found.RevokedAt != "" {
		// Key no longer exists (revoked or deleted).
		resp.State.RemoveResource(ctx)
		return
	}

	applyAPIKeyToState(&state, *found, false)

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
	projectID := ""
	if !state.ProjectID.IsNull() && !state.ProjectID.IsUnknown() {
		projectID = state.ProjectID.ValueString()
	}

	_, err := r.client.Post(ctx, apiKeyRevokePath(projectID, state.ID.ValueString()), nil)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Error revoking API key", err.Error())
	}
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	switch len(parts) {
	case 1:
		resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	case 2:
		if parts[0] == "" || parts[1] == "" {
			resp.Diagnostics.AddError(
				"Invalid API Key Import ID",
				"Import ID must be either key_id or project_id/key_id.",
			)
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	default:
		resp.Diagnostics.AddError(
			"Invalid API Key Import ID",
			"Import ID must be either key_id or project_id/key_id.",
		)
	}
}

func (r *apiKeyResource) findAPIKey(ctx context.Context, id string, projectID string) (*apiKeyAPIModel, error) {
	cursor := ""
	for {
		params := url.Values{}
		params.Set("status", "all")
		params.Set("limit", "200")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, err := r.client.Get(ctx, apiKeyCollectionPath(projectID)+"?"+params.Encode())
		if err != nil {
			return nil, err
		}

		var apiResp apiKeyListResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return nil, fmt.Errorf("parsing API key list response: %w", err)
		}

		for i := range apiResp.Items {
			if apiResp.Items[i].ID == id {
				return &apiResp.Items[i], nil
			}
		}

		if apiResp.NextCursor == "" {
			return nil, nil
		}
		cursor = apiResp.NextCursor
	}
}

func applyAPIKeyToState(state *apiKeyResourceModel, key apiKeyAPIModel, includeRawKey bool) {
	state.ID = stringValueOrNull(key.ID)
	state.ProjectID = stringValueOrNull(key.ProjectID)
	state.Name = stringValueOrNull(key.Name)
	state.Description = stringValueOrNull(key.Description)
	state.Scope = stringValueOrNull(key.Scope)
	state.CreatedBy = stringValueOrNull(key.CreatedBy)
	state.Prefix = stringValueOrNull(key.Prefix)
	state.CreatedAt = stringValueOrNull(key.CreatedAt)
	state.RevokedAt = stringValueOrNull(key.RevokedAt)
	state.LastUsedAt = stringValueOrNull(key.LastUsedAt)
	state.ExpiresAt = stringValueOrNull(key.ExpiresAt)
	if key.RevokedAt != "" {
		state.Status = types.StringValue("revoked")
	} else {
		state.Status = types.StringValue("active")
	}
	if includeRawKey {
		state.RawKey = stringValueOrNull(key.RawKey)
	}
}

func apiKeyCollectionPath(projectID string) string {
	if projectID == "" {
		return "/v1/api-keys"
	}
	return fmt.Sprintf("/v1/projects/%s/api-keys", url.PathEscape(projectID))
}

func apiKeyRevokePath(projectID, keyID string) string {
	if projectID == "" {
		return fmt.Sprintf("/v1/api-keys/%s/revoke", url.PathEscape(keyID))
	}
	return fmt.Sprintf("/v1/projects/%s/api-keys/%s/revoke", url.PathEscape(projectID), url.PathEscape(keyID))
}

func stringValueOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func validAPIKeyScope(scope string) bool {
	switch scope {
	case "admin", "client", "approval":
		return true
	default:
		return false
	}
}
