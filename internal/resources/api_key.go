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
				Computed:    true,
				Description: "Project the key belongs to: always the project of the provider's API key, because Keel creates, lists and revokes keys within that project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
	data := configuredProviderData(req, resp)
	if data == nil {
		return
	}
	if data.APIKey == nil {
		resp.Diagnostics.AddError(missingAPIKeySummary, missingAPIKeyDetail)
		return
	}
	r.client = data.APIKey
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

	body, err := r.client.Post(ctx, apiKeysPath, apiReq)
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

	// No single-key GET endpoint — use the list endpoint and filter by ID.
	keyID := state.ID.ValueString()
	found, listedProjectID, err := r.findAPIKey(ctx, keyID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing API keys", err.Error())
		return
	}

	stateProjectID := state.ProjectID.ValueString()
	if found == nil {
		if stateProjectID != "" && listedProjectID != "" && listedProjectID != stateProjectID {
			// Keel lists only the provider key's project, so absence here says
			// nothing about the key. Refuse rather than drop a live key from state.
			resp.Diagnostics.AddError(wrongProjectSummary, wrongProjectDetail(keyID, stateProjectID, listedProjectID))
			return
		}
		// Key no longer exists.
		resp.State.RemoveResource(ctx)
		return
	}
	if stateProjectID != "" && found.ProjectID != stateProjectID {
		// Only reachable through a project_id/key_id import naming the wrong project.
		resp.Diagnostics.AddError(
			"API key belongs to a different project",
			fmt.Sprintf("The import ID names project %s, but API key %s belongs to project %s.", stateProjectID, keyID, found.ProjectID),
		)
		return
	}
	if found.RevokedAt != "" {
		// Key has been revoked.
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

	// API uses POST revoke, not DELETE. Revoking an already revoked key succeeds.
	keyID := state.ID.ValueString()
	_, err := r.client.Post(ctx, apiKeyRevokePath(keyID), nil)
	if err == nil {
		return
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		resp.Diagnostics.AddError("Error revoking API key", err.Error())
		return
	}

	// 404: the key is not in the provider key's project. It is gone only if
	// state records it in that same project.
	stateProjectID := state.ProjectID.ValueString()
	if stateProjectID == "" {
		return
	}
	listedProjectID, listErr := r.providerProjectID(ctx)
	if listErr != nil {
		resp.Diagnostics.AddError(
			"Error revoking API key",
			fmt.Sprintf("%s\n\nChecking which project the provider's API key belongs to also failed: %s", err, listErr),
		)
		return
	}
	if listedProjectID != "" && listedProjectID != stateProjectID {
		resp.Diagnostics.AddError(wrongProjectSummary, wrongProjectDetail(keyID, stateProjectID, listedProjectID))
	}
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	switch {
	case len(parts) == 1 && parts[0] != "":
		resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	case len(parts) == 2 && parts[0] != "" && parts[1] != "":
		// Legacy project_id/key_id form. The project must be the provider API
		// key's project; Read checks it.
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	default:
		resp.Diagnostics.AddError(
			"Invalid API Key Import ID",
			"Import ID must be the API key ID (key_id). The legacy project_id/key_id form is also accepted when project_id is the provider API key's project.",
		)
	}
}

// findAPIKey pages through the provider key's project and returns the key
// with the given ID (nil when absent) and the project the listed keys belong to.
func (r *apiKeyResource) findAPIKey(ctx context.Context, id string) (*apiKeyAPIModel, string, error) {
	cursor := ""
	listedProjectID := ""
	for {
		params := url.Values{}
		params.Set("status", "all")
		params.Set("limit", "200")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, err := r.client.Get(ctx, apiKeysPath+"?"+params.Encode())
		if err != nil {
			return nil, "", err
		}

		var apiResp apiKeyListResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return nil, "", fmt.Errorf("parsing API key list response: %w", err)
		}

		for i := range apiResp.Items {
			if listedProjectID == "" {
				listedProjectID = apiResp.Items[i].ProjectID
			}
			if apiResp.Items[i].ID == id {
				return &apiResp.Items[i], listedProjectID, nil
			}
		}

		if apiResp.NextCursor == "" {
			return nil, listedProjectID, nil
		}
		cursor = apiResp.NextCursor
	}
}

// providerProjectID returns the project of the provider's API key, read from
// the first key Keel lists (the list always includes the calling key).
func (r *apiKeyResource) providerProjectID(ctx context.Context) (string, error) {
	body, err := r.client.Get(ctx, apiKeysPath+"?status=all&limit=1")
	if err != nil {
		return "", err
	}
	var apiResp apiKeyListResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("parsing API key list response: %w", err)
	}
	if len(apiResp.Items) == 0 {
		return "", nil
	}
	return apiResp.Items[0].ProjectID, nil
}

const wrongProjectSummary = "API key belongs to a different project"

func wrongProjectDetail(keyID, stateProjectID, providerProjectID string) string {
	return fmt.Sprintf(
		"State records API key %s in project %s, but the provider's API key belongs to project %s. "+
			"Keel manages API keys only within the project of the API key making the request, so this key cannot be read or revoked with the current credential. "+
			"Configure the provider with an admin-scope API key from project %s, or run `terraform state rm` if the key is managed elsewhere.",
		keyID, stateProjectID, providerProjectID, stateProjectID,
	)
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

const apiKeysPath = "/v1/api-keys"

func apiKeyRevokePath(keyID string) string {
	return fmt.Sprintf("%s/%s/revoke", apiKeysPath, url.PathEscape(keyID))
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
