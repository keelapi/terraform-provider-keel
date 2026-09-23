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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/client"
	"github.com/keelapi/terraform-provider-keel/internal/timestamp"
	"github.com/keelapi/terraform-provider-keel/internal/validators"
)

var _ resource.Resource = &apiKeyResource{}
var _ resource.ResourceWithImportState = &apiKeyResource{}
var _ resource.ResourceWithModifyPlan = &apiKeyResource{}

type apiKeyResource struct {
	client *client.Client
}

type apiKeyResourceModel struct {
	ID               types.String      `tfsdk:"id"`
	ProjectID        types.String      `tfsdk:"project_id"`
	Name             types.String      `tfsdk:"name"`
	Description      types.String      `tfsdk:"description"`
	Scope            types.String      `tfsdk:"scope"`
	AgentPrincipalID types.String      `tfsdk:"agent_principal_id"`
	CreatedBy        types.String      `tfsdk:"created_by"`
	Prefix           types.String      `tfsdk:"prefix"`
	RawKey           types.String      `tfsdk:"raw_key"`
	CreatedAt        types.String      `tfsdk:"created_at"`
	RevokedAt        types.String      `tfsdk:"revoked_at"`
	LastUsedAt       types.String      `tfsdk:"last_used_at"`
	ExpiresAt        timestamp.RFC3339 `tfsdk:"expires_at"`
	Status           types.String      `tfsdk:"status"`
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
				Description: "Key scope: \"admin\", \"client\", or \"approval\". Default: \"admin\". An approval-scope key exists only after dual-control approval, which Terraform cannot wait for: the apply fails with the pending change's ID and nothing is stored.",
				Validators: []validator.String{
					validators.OneOf("admin", "client", "approval"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"agent_principal_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Agent principal to bind the key to, making it that agent's execution credential. Cannot be combined with scope \"approval\". When the provider's own API key is bound to an agent, Keel binds new keys to the same agent.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
				CustomType:  timestamp.RFC3339Type{},
				Optional:    true,
				Computed:    true,
				Description: "Expiration timestamp, if configured: RFC 3339 with a UTC offset, such as \"2027-01-01T00:00:00Z\". Keel may return the same instant spelled differently; that is not a change.",
				Validators: []validator.String{
					validators.RFC3339(),
				},
				PlanModifiers: []planmodifier.String{
					timestamp.UseStateForSameInstant(),
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
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
	ID               string `json:"id,omitempty"`
	ProjectID        string `json:"project_id,omitempty"`
	Name             string `json:"name,omitempty"`
	Description      string `json:"description,omitempty"`
	Scope            string `json:"scope,omitempty"`
	AgentPrincipalID string `json:"agent_principal_id,omitempty"`
	CreatedBy        string `json:"created_by,omitempty"`
	Prefix           string `json:"prefix,omitempty"`
	RawKey           string `json:"raw_key,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	RevokedAt        string `json:"revoked_at,omitempty"`
	LastUsedAt       string `json:"last_used_at,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
}

type apiKeyCreateRequest struct {
	Name             string `json:"name,omitempty"`
	Description      string `json:"description,omitempty"`
	Scope            string `json:"scope,omitempty"`
	AgentPrincipalID string `json:"agent_principal_id,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
}

// pendingAPIKeyCreateResponse is the part of Keel's 202 response for an
// approval-scope key awaiting dual-control approval (PendingApiKeyCreateResponse)
// that the provider reports. Its raw_key is deliberately not decoded.
type pendingAPIKeyCreateResponse struct {
	PendingChangeID string `json:"pending_change_id"`
	Status          string `json:"status"`
	ExpiresAt       string `json:"expires_at"`
}

// parsePendingAPIKeyCreate reports whether a create response is a pending
// approval rather than a key.
func parsePendingAPIKeyCreate(status int, body []byte) (*pendingAPIKeyCreateResponse, bool) {
	var pending pendingAPIKeyCreateResponse
	if err := json.Unmarshal(body, &pending); err != nil {
		return &pending, status == http.StatusAccepted
	}
	if status == http.StatusAccepted || pending.PendingChangeID != "" {
		return &pending, true
	}
	return nil, false
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

	apiReq := apiKeyCreateRequest{
		Scope: plan.Scope.ValueString(),
	}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		apiReq.Name = plan.Name.ValueString()
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		apiReq.Description = plan.Description.ValueString()
	}
	if !plan.AgentPrincipalID.IsNull() && !plan.AgentPrincipalID.IsUnknown() {
		apiReq.AgentPrincipalID = plan.AgentPrincipalID.ValueString()
	}
	if !plan.ExpiresAt.IsNull() && !plan.ExpiresAt.IsUnknown() {
		apiReq.ExpiresAt = plan.ExpiresAt.ValueString()
	}

	body, status, err := r.client.PostWithStatus(ctx, apiKeysPath, apiReq)
	if err != nil {
		detail := err.Error()
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.Code == "approval.authority_configuration_required" {
			detail += "\n\nApproval-scope keys can only be requested when the project enforces dual control, and each one then waits for a second approver before it is minted."
		}
		resp.Diagnostics.AddError("Error creating API key", detail)
		return
	}

	// An approval-scope key under enforced dual control is not created yet:
	// Keel answers 202 with a pending change. Never store it as a key.
	if pending, ok := parsePendingAPIKeyCreate(status, body); ok {
		resp.Diagnostics.AddError(
			"API key is waiting for dual-control approval",
			fmt.Sprintf("Keel accepted the request for an approval-scope API key as pending change %s (status %q). "+
				"A second approver from the project's approver group must approve it before the key exists; the request expires at %s.\n\n"+
				"Terraform cannot manage a key that does not exist yet, so nothing was saved to state, and the key's secret "+
				"(returned only in this response) was discarded rather than stored. If the change is approved, the key's secret "+
				"cannot be recovered from Terraform: reject the pending change or let it expire, and request approval-scope keys "+
				"directly with POST /v1/api-keys, keeping the raw_key it returns until the key is approved.",
				pending.PendingChangeID, pending.Status, pending.ExpiresAt),
		)
		return
	}

	var apiResp apiKeyAPIModel
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}
	if apiResp.ID == "" {
		resp.Diagnostics.AddError("Error parsing response", fmt.Sprintf("Keel answered HTTP %d without an API key id.", status))
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

// ModifyPlan plans no change when the configuration matches the key: every
// configured attribute equals state, with expires_at compared by instant.
// Without this, a configured expires_at spelled differently from state (for
// example after import, which stores Keel's spelling) makes the framework mark
// the unset computed attributes unknown, planning an update of an unchanged
// key, or a replacement when agent_principal_id is unset.
func (r *apiKeyResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return // create or destroy
	}
	var config, plan, state apiKeyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if apiKeyConfigMatchesState(config, plan, state) {
		resp.Plan.Raw = req.State.Raw.Copy()
	}
}

func apiKeyConfigMatchesState(config, plan, state apiKeyResourceModel) bool {
	if !config.Name.Equal(state.Name) || !config.Description.Equal(state.Description) {
		return false
	}
	// scope defaults to "admin": compare the planned value.
	if !plan.Scope.Equal(state.Scope) {
		return false
	}
	// Optional and computed: unset means "whatever the key has".
	if !config.AgentPrincipalID.IsNull() && !config.AgentPrincipalID.Equal(state.AgentPrincipalID) {
		return false
	}
	if !config.ExpiresAt.IsNull() {
		if config.ExpiresAt.IsUnknown() || state.ExpiresAt.IsNull() || state.ExpiresAt.IsUnknown() ||
			!timestamp.SameInstant(config.ExpiresAt.ValueString(), state.ExpiresAt.ValueString()) {
			return false
		}
	}
	return true
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
	state.AgentPrincipalID = stringValueOrNull(key.AgentPrincipalID)
	state.CreatedBy = stringValueOrNull(key.CreatedBy)
	state.Prefix = stringValueOrNull(key.Prefix)
	state.CreatedAt = stringValueOrNull(key.CreatedAt)
	state.RevokedAt = stringValueOrNull(key.RevokedAt)
	state.LastUsedAt = stringValueOrNull(key.LastUsedAt)
	if key.ExpiresAt == "" {
		state.ExpiresAt = timestamp.NewNull()
	} else {
		state.ExpiresAt = timestamp.NewValue(key.ExpiresAt)
	}
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
