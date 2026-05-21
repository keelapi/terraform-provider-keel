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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

var _ resource.Resource = &organizationMemberResource{}
var _ resource.ResourceWithImportState = &organizationMemberResource{}

type organizationMemberResource struct {
	client *client.Client
}

type organizationMemberResourceModel struct {
	ID        types.String `tfsdk:"id"`
	OrgID     types.String `tfsdk:"org_id"`
	UserID    types.String `tfsdk:"user_id"`
	Role      types.String `tfsdk:"role"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func NewOrganizationMemberResource() resource.Resource {
	return &organizationMemberResource{}
}

func (r *organizationMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_member"
}

func (r *organizationMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Keel organization member role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Organization membership ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"org_id": schema.StringAttribute{
				Required:    true,
				Description: "Keel organization ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "Keel user ID to grant membership to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Required:    true,
				Description: "Organization role to assign to the user.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Membership creation timestamp.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *organizationMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type organizationMemberAPIModel struct {
	ID        string `json:"id,omitempty"`
	OrgID     string `json:"org_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Role      string `json:"role,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type organizationMemberCreateRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type organizationMemberPatchRequest struct {
	Role string `json:"role"`
}

type organizationMemberListResponse struct {
	Items []organizationMemberAPIModel `json:"items"`
}

type pendingChangeResponse struct {
	ID         string `json:"id,omitempty"`
	Status     string `json:"status,omitempty"`
	TargetType string `json:"target_type,omitempty"`
	TargetRef  string `json:"target_ref,omitempty"`
}

func (r *organizationMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := organizationMemberCreateRequest{
		UserID: plan.UserID.ValueString(),
		Role:   plan.Role.ValueString(),
	}

	body, err := r.client.Post(ctx, organizationMembersPath(plan.OrgID.ValueString()), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization member", err.Error())
		return
	}

	apiResp, pending, err := parseOrganizationMemberMutation(body)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}
	if pending != nil {
		resp.Diagnostics.AddError(
			"Organization member change is pending approval",
			fmt.Sprintf("Keel returned pending change %s with status %s for %s. Terraform cannot manage this resource until the change is approved.", pending.ID, pending.Status, pending.TargetRef),
		)
		return
	}

	applyOrganizationMemberToState(&plan, *apiResp)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *organizationMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, err := r.findOrganizationMember(ctx, state.OrgID.ValueString(), state.UserID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing organization members", err.Error())
		return
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	applyOrganizationMemberToState(&state, *found)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *organizationMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan organizationMemberResourceModel
	var state organizationMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := organizationMemberPatchRequest{
		Role: plan.Role.ValueString(),
	}

	body, err := r.client.Patch(ctx, organizationMemberPath(state.OrgID.ValueString(), state.UserID.ValueString()), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization member", err.Error())
		return
	}

	apiResp, pending, err := parseOrganizationMemberMutation(body)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}
	if pending != nil {
		resp.Diagnostics.AddError(
			"Organization member change is pending approval",
			fmt.Sprintf("Keel returned pending change %s with status %s for %s. Terraform cannot manage this resource until the change is approved.", pending.ID, pending.Status, pending.TargetRef),
		)
		return
	}

	applyOrganizationMemberToState(&plan, *apiResp)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *organizationMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, organizationMemberPath(state.OrgID.ValueString(), state.UserID.ValueString()))
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Error deleting organization member", err.Error())
	}
}

func (r *organizationMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Organization Member Import ID",
			"Import ID must be org_id/user_id.",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("org_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), parts[1])...)
}

func (r *organizationMemberResource) findOrganizationMember(ctx context.Context, orgID, userID string) (*organizationMemberAPIModel, error) {
	body, err := r.client.Get(ctx, organizationMembersPath(orgID))
	if err != nil {
		return nil, err
	}

	var apiResp organizationMemberListResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing organization member list response: %w", err)
	}

	for i := range apiResp.Items {
		if apiResp.Items[i].UserID == userID {
			return &apiResp.Items[i], nil
		}
	}
	return nil, nil
}

func parseOrganizationMemberMutation(body []byte) (*organizationMemberAPIModel, *pendingChangeResponse, error) {
	var member organizationMemberAPIModel
	if err := json.Unmarshal(body, &member); err != nil {
		return nil, nil, err
	}
	if member.ID != "" && member.OrgID != "" && member.UserID != "" {
		return &member, nil, nil
	}

	var pending pendingChangeResponse
	if err := json.Unmarshal(body, &pending); err != nil {
		return nil, nil, err
	}
	if pending.ID != "" && pending.Status != "" {
		return nil, &pending, nil
	}

	return nil, nil, fmt.Errorf("response did not contain an organization member")
}

func applyOrganizationMemberToState(state *organizationMemberResourceModel, member organizationMemberAPIModel) {
	state.ID = stringValueOrNull(member.ID)
	state.OrgID = stringValueOrNull(member.OrgID)
	state.UserID = stringValueOrNull(member.UserID)
	state.Role = stringValueOrNull(member.Role)
	state.CreatedAt = stringValueOrNull(member.CreatedAt)
}

func organizationMembersPath(orgID string) string {
	return fmt.Sprintf("/v1/organizations/%s/members", url.PathEscape(orgID))
}

func organizationMemberPath(orgID, userID string) string {
	return fmt.Sprintf("%s/%s", organizationMembersPath(orgID), url.PathEscape(userID))
}
