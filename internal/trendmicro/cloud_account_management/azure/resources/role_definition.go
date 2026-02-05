package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/config"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	ErrorFailedToGetSubscription = "Failed to get subscription"
	APIVersion                   = "2022-04-01"
	AzureManagementScope         = "https://management.azure.com/.default"
	RoleDefinitionURLTemplate    = "https://management.azure.com/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s?api-version=%s"
)

type RoleDefinition struct {
	client *api.CamClient
}

type customRoleDefinitionResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Scope            types.String `tfsdk:"scope"`
	Description      types.String `tfsdk:"description"`
	Features         types.Set    `tfsdk:"features"`
	SubscriptionId   types.String `tfsdk:"subscription_id"`
	AssignableScopes types.Set    `tfsdk:"assignable_scopes"`
}

func NewRoleDefinition() resource.Resource {
	return &RoleDefinition{
		client: &api.CamClient{},
	}
}

func (r *RoleDefinition) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_ROLE_DEFINITION
}

func (r *RoleDefinition) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Trend Micro Vision One CAM Azure Role Definition resource. Creates a custom Azure role with the [necessary permissions](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-azure-sub-required-permissions) for Vision One Cloud Account Management.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for the Trend Vision One CAM custom role definition.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the Trend Vision One CAM custom role definition.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "Scope where the Trend Vision One CAM custom role definition applies, typically a subscription.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the Trend Vision One CAM custom role definition.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"features": schema.SetAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "Set of features associated with the Trend Vision One CAM custom role definition. The role will include all permissions required by the specified features according to the Trend Vision One Azure required permissions documentation.",
				Optional:            true,
			},
			"assignable_scopes": schema.SetAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "Set of scopes where the role can be assigned. Defaults to the subscription scope if not provided. For management group deployments, include all member subscription scopes to enable cross-subscription role assignments. Example: [\"/subscriptions/sub-id-1\", \"/subscriptions/sub-id-2\"] or [\"/providers/Microsoft.Management/managementGroups/mg-id\"]",
				Optional:            true,
				Computed:            true,
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the subscription.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *RoleDefinition) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customRoleDefinitionResourceModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	subID := plan.SubscriptionId.ValueString()

	roleDefinitionID := uuid.New().String()
	roleName := config.AZURE_CUSTOM_ROLE_NAME + subID + "-" + generateRandomString(4)
	roleScope := config.AZURE_CUSTOM_ROLE_SCOPE + subID
	roleDescription := config.AZURE_CUSTOM_ROLE_DESCRIPTION

	// Extract assignable scopes from plan, or default to subscription scope
	var assignableScopes []string
	if !plan.AssignableScopes.IsNull() && !plan.AssignableScopes.IsUnknown() {
		diags := plan.AssignableScopes.ElementsAs(ctx, &assignableScopes, false)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
	}
	if len(assignableScopes) == 0 {
		// Default to subscription scope if not provided
		assignableScopes = []string{roleScope}
	}

	if err := r.createRoleDefinition(ctx, subID, roleDefinitionID, roleName, roleDescription, assignableScopes); err != nil {
		resp.Diagnostics.AddError("[Role Definition][Create] Failed to create role definition", err.Error())
		return
	}

	plan.ID = types.StringValue(roleDefinitionID)
	plan.Name = types.StringValue(roleName)
	plan.Scope = types.StringValue(roleScope)
	plan.Description = types.StringValue(roleDescription)

	// Set assignable_scopes in state
	assignableScopesSet, diags := types.SetValueFrom(ctx, types.StringType, assignableScopes)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	plan.AssignableScopes = assignableScopesSet

	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *RoleDefinition) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customRoleDefinitionResourceModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	subID, err := api.GetDefaultSubscription()
	if err != nil {
		resp.Diagnostics.AddError("[Role Definition][Read] Failed to get subscription", err.Error())
		return
	}

	client, diags := api.GetAzureClients(ctx, subID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	roleDefinitionID := state.ID.ValueString()
	scope := state.Scope.ValueString()

	roleDefinition, err := client.RoleClient.Get(ctx, scope, roleDefinitionID, nil)
	if err != nil {
		if strings.Contains(err.Error(), `"code": "RoleDefinitionDoesNotExist"`) {
			resp.State.RemoveResource(ctx)
			return
		}
		// Otherwise report the error
		resp.Diagnostics.AddError("[Role Definition][Read] Failed to get role definition", err.Error())
		return
	}

	if roleDefinition.Properties != nil {
		state.Name = types.StringPointerValue(roleDefinition.Properties.RoleName)
		state.Description = types.StringPointerValue(roleDefinition.Properties.Description)

		// Read assignable scopes from Azure
		if roleDefinition.Properties.AssignableScopes != nil {
			var scopes []string
			for _, scope := range roleDefinition.Properties.AssignableScopes {
				if scope != nil {
					scopes = append(scopes, *scope)
				}
			}
			assignableScopesSet, diags := types.SetValueFrom(ctx, types.StringType, scopes)
			if diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			state.AssignableScopes = assignableScopesSet
		}
	}

	if diags := resp.State.Set(ctx, state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *RoleDefinition) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state customRoleDefinitionResourceModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	subID, err := api.GetDefaultSubscription()
	if err != nil {
		resp.Diagnostics.AddError("[Role Definition][Update] Failed to get subscription", err.Error())
		return
	}

	roleDefinitionID := state.ID.ValueString()
	scope := state.Scope.ValueString()
	roleName := state.Name.ValueString()
	roleDescription := state.Description.ValueString()

	// Extract assignable scopes from plan
	var assignableScopes []string
	if !plan.AssignableScopes.IsNull() && !plan.AssignableScopes.IsUnknown() {
		diags := plan.AssignableScopes.ElementsAs(ctx, &assignableScopes, false)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
	}
	if len(assignableScopes) == 0 {
		// Default to subscription scope if not provided
		assignableScopes = []string{scope}
	}

	if err := r.updateRoleDefinition(ctx, subID, roleDefinitionID, roleName, roleDescription, assignableScopes); err != nil {
		resp.Diagnostics.AddError("[Role Definition][Update] Failed to update role definition", err.Error())
		return
	}

	// Update assignable_scopes in state
	assignableScopesSet, diags := types.SetValueFrom(ctx, types.StringType, assignableScopes)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	plan.AssignableScopes = assignableScopesSet

	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *RoleDefinition) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customRoleDefinitionResourceModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	subID := state.SubscriptionId.ValueString()
	roleDefinitionID := state.ID.ValueString()

	if err := r.deleteRoleDefinition(ctx, subID, roleDefinitionID); err != nil {
		resp.Diagnostics.AddError("[Role Definition][Delete] Failed to delete role definition", err.Error())
	}
}

func (r *RoleDefinition) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *trendmicro.Client, got something else.",
		)
		return
	}

	r.client = &api.CamClient{
		Client: client,
	}
	tflog.Debug(ctx, "[Role Definition] resource configured successfully")
}

func (r *RoleDefinition) buildRoleDefinitionBody(roleName, roleDescription string, assignableScopes []string) map[string]any {
	return map[string]any{
		"properties": map[string]any{
			"roleName":         roleName,
			"description":      roleDescription,
			"assignableScopes": assignableScopes,
			"permissions": []map[string]any{
				{
					"actions":     config.AZURE_CUSTOM_ROLE_ACTIONS,
					"dataActions": config.AZURE_CUSTOM_ROLE_DATA_ACTIONS,
				},
			},
		},
	}
}

func (r *RoleDefinition) createRoleDefinition(ctx context.Context, subID, roleDefinitionID, roleName, roleDescription string, assignableScopes []string) error {
	token, err := r.getAzureToken(ctx)
	if err != nil {
		return err
	}

	body := r.buildRoleDefinitionBody(roleName, roleDescription, assignableScopes)
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}
	return r.doRoleDefinitionRequest(ctx, "PUT", subID, roleDefinitionID, token, bytes.NewReader(jsonBody))
}

func (r *RoleDefinition) deleteRoleDefinition(ctx context.Context, subID, roleDefinitionID string) error {
	token, err := r.getAzureToken(ctx)
	if err != nil {
		return err
	}
	return r.doRoleDefinitionRequest(ctx, "DELETE", subID, roleDefinitionID, token, nil)
}

func (r *RoleDefinition) doRoleDefinitionRequest(ctx context.Context, method, subID, roleDefinitionID, token string, body *bytes.Reader) error {
	url := fmt.Sprintf(RoleDefinitionURLTemplate, subID, roleDefinitionID, APIVersion)
	var bodyReader io.Reader
	if body != nil {
		bodyReader = body
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return fmt.Errorf("failed with Azure API error: status %d", res.StatusCode)
	}

	return nil
}

func (r *RoleDefinition) getAzureToken(ctx context.Context) (string, error) {
	cred, err := api.GetAzureCredential()
	if err != nil {
		return "", fmt.Errorf("failed to get Azure credential: %w", err)
	}

	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{AzureManagementScope},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get Azure token: %w", err)
	}

	return token.Token, nil
}

func (r *RoleDefinition) updateRoleDefinition(ctx context.Context, subID, roleDefinitionID, roleName, roleDescription string, assignableScopes []string) error {
	token, err := r.getAzureToken(ctx)
	if err != nil {
		return err
	}

	body := r.buildRoleDefinitionBody(roleName, roleDescription, assignableScopes)
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}
	return r.doRoleDefinitionRequest(ctx, "PUT", subID, roleDefinitionID, token, bytes.NewReader(jsonBody))
}
