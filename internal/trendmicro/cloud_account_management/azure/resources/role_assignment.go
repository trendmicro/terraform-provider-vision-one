package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/config"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	subscriptionScopeFormat   = "/subscriptions/%s"
	azureManagementScope      = "https://management.azure.com/.default"
	azureManagementBaseURL    = "https://management.azure.com"
	roleAssignmentAPIVersion  = "2022-04-01"
	roleAssignmentURLFormat   = "%s%s/providers/Microsoft.Authorization/roleAssignments/%s?api-version=%s"
	errorStatusResponseFormat = "Status: %d\nResponse: %s"
)

type roleAssignmentResource struct {
	client *api.CamClient
}

type roleAssignmentResourceModel struct {
	ID               types.String `tfsdk:"id"`
	SubscriptionId   types.String `tfsdk:"subscription_id"`
	RoleDefinitionId types.String `tfsdk:"role_definition_id"`
	PrincipalId      types.String `tfsdk:"principal_id"`
}

func NewRoleAssignmentResource() resource.Resource {
	return &roleAssignmentResource{
		client: &api.CamClient{},
	}
}

func (r *roleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_ROLE_ASSIGNMENT
}

func (r *roleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Trend Micro Vision One CAM Azure Role Assignment resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for the role assignment.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the subscription.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role_definition_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the role definition to assign.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"principal_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the principal to assign the role to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *roleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleAssignmentResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Azure credential ", err.Error())
		return
	}

	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{azureManagementScope},
	})
	if err != nil {
		resp.Diagnostics.AddError("Token Acquisition Error: ", err.Error())
		return
	}

	assignmentId := uuid.New().String()
	scope := fmt.Sprintf(subscriptionScopeFormat, plan.SubscriptionId.ValueString())
	url := fmt.Sprintf(roleAssignmentURLFormat,
		azureManagementBaseURL, scope, assignmentId, roleAssignmentAPIVersion)

	payload := map[string]any{
		"properties": map[string]any{
			"roleDefinitionId": fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", plan.SubscriptionId.ValueString(), plan.RoleDefinitionId.ValueString()),
			"principalId":      plan.PrincipalId.ValueString(),
			"principalType":    "ServicePrincipal",
		},
	}

	body, _ := json.Marshal(payload)
	reqHttp, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	reqHttp.Header.Set("Authorization", "Bearer "+token.Token)
	reqHttp.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("HTTP Request Failed", err.Error())
		return
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		content, _ := io.ReadAll(res.Body)
		resp.Diagnostics.AddError("Azure API Error", fmt.Sprintf(errorStatusResponseFormat, res.StatusCode, content))
		return
	}

	plan.ID = types.StringValue(assignmentId)
	diags = resp.State.Set(ctx, plan)

	resp.Diagnostics.Append(diags...)
}

func (r *roleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleAssignmentResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Azure credential ", err.Error())
		return
	}

	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{azureManagementScope},
	})
	if err != nil {
		resp.Diagnostics.AddError("Token Acquisition Error: ", err.Error())
		return
	}

	scope := fmt.Sprintf(subscriptionScopeFormat, state.SubscriptionId.ValueString())
	url := fmt.Sprintf(roleAssignmentURLFormat,
		azureManagementBaseURL, scope, state.ID.ValueString(), roleAssignmentAPIVersion)

	reqHttp, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	reqHttp.Header.Set("Authorization", "Bearer "+token.Token)

	res, err := http.DefaultClient.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("Read Request Failed", err.Error())
		return
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if res.StatusCode >= 300 {
		content, _ := io.ReadAll(res.Body)
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf(errorStatusResponseFormat, res.StatusCode, content))
	}
}

func (r *roleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	deleteResp := resource.DeleteResponse{Diagnostics: resp.Diagnostics}
	r.Delete(ctx, resource.DeleteRequest{State: req.State}, &deleteResp)

	createResp := resource.CreateResponse{Diagnostics: resp.Diagnostics, State: resp.State}
	r.Create(ctx, resource.CreateRequest{Plan: req.Plan}, &createResp)
}

func (r *roleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleAssignmentResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Azure credential ", err.Error())
		return
	}

	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{azureManagementScope},
	})
	if err != nil {
		resp.Diagnostics.AddError("Token Acquisition Error: ", err.Error())
		return
	}

	scope := fmt.Sprintf(subscriptionScopeFormat, state.SubscriptionId.ValueString())
	url := fmt.Sprintf(roleAssignmentURLFormat,
		azureManagementBaseURL, scope, state.ID.ValueString(), roleAssignmentAPIVersion)

	reqHttp, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	reqHttp.Header.Set("Authorization", "Bearer "+token.Token)

	res, err := http.DefaultClient.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("Delete Request Failed", err.Error())
		return
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 && res.StatusCode != 404 {
		content, _ := io.ReadAll(res.Body)
		resp.Diagnostics.AddError("Delete Error", fmt.Sprintf(errorStatusResponseFormat, res.StatusCode, content))
	}
}
