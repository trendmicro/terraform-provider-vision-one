package resources

import (
	"context"
	"fmt"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
)

var _ resource.Resource = &LegacyCleanupServiceAccount{}

type LegacyCleanupServiceAccount struct{}

type legacyCleanupServiceAccountModel struct {
	ID                  types.String `tfsdk:"id"`
	ProjectID           types.String `tfsdk:"project_id"`
	ServiceAccountKey   types.String `tfsdk:"service_account_key"`
	Deleted             types.Bool   `tfsdk:"deleted"`
	ServiceAccountEmail types.String `tfsdk:"service_account_email"`
	KeysDeletedCount    types.Int64  `tfsdk:"keys_deleted_count"`
	DeletionTimestamp   types.String `tfsdk:"deletion_timestamp"`
	CleanupStatus       types.String `tfsdk:"cleanup_status"`
	CleanupError        types.String `tfsdk:"cleanup_error"`
}

func NewLegacyCleanupServiceAccount() resource.Resource {
	return &LegacyCleanupServiceAccount{}
}

func (r *LegacyCleanupServiceAccount) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_LEGACY_CLEANUP_SERVICE_ACCOUNT
}

func (r *LegacyCleanupServiceAccount) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deletes the legacy GCP service account (`vision-one-service-account`) and all its keys, as created by the Terraform Package Solution. Returns `cleanup_status = \"not_found\"` if the service account does not exist.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project ID containing the legacy service account.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account_key": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded JSON service account key used to authenticate with GCP.",
				Optional:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"deleted": schema.BoolAttribute{
				MarkdownDescription: "Whether the service account was deleted.",
				Computed:            true,
			},
			"service_account_email": schema.StringAttribute{
				MarkdownDescription: "Email of the detected legacy service account.",
				Computed:            true,
			},
			"keys_deleted_count": schema.Int64Attribute{
				MarkdownDescription: "Number of service account keys that were deleted.",
				Computed:            true,
			},
			"deletion_timestamp": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when cleanup was performed.",
				Computed:            true,
			},
			"cleanup_status": schema.StringAttribute{
				MarkdownDescription: "Status: deleted, not_found, or failed.",
				Computed:            true,
			},
			"cleanup_error": schema.StringAttribute{
				MarkdownDescription: "Error message if cleanup failed.",
				Computed:            true,
			},
		},
	}
}

func (r *LegacyCleanupServiceAccount) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan legacyCleanupServiceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", config.LEGACY_GCP_SERVICE_ACCOUNT_NAME, projectID)

	plan.ID = types.StringValue(projectID)
	plan.Deleted = types.BoolValue(false)
	plan.ServiceAccountEmail = types.StringValue(saEmail)
	plan.KeysDeletedCount = types.Int64Value(0)
	plan.DeletionTimestamp = types.StringValue("")
	plan.CleanupError = types.StringValue("")

	clientOptions := []option.ClientOption{}
	if serviceAccountKey := plan.ServiceAccountKey.ValueString(); serviceAccountKey != "" {
		clientOption, err := newClientOptionFromEncodedServiceAccountKey(ctx, serviceAccountKey)
		if err != nil {
			resp.Diagnostics.AddError("[SA Cleanup] Invalid service account key", err.Error())
			return
		}

		clientOptions = append(clientOptions, clientOption)
	}

	iamSvc, err := iam.NewService(ctx, clientOptions...)
	if err != nil {
		resp.Diagnostics.AddError("[SA Cleanup] Failed to create IAM client", err.Error())
		return
	}

	saName := fmt.Sprintf("projects/%s/serviceAccounts/%s", projectID, saEmail)
	_, err = iamSvc.Projects.ServiceAccounts.Get(saName).Context(ctx).Do()
	if err != nil {
		if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == 404 {
			tflog.Info(ctx, fmt.Sprintf("[SA Cleanup] Service account not found: %s", saEmail))
			plan.CleanupStatus = types.StringValue("not_found")
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			return
		}
		plan.CleanupStatus = types.StringValue("failed")
		plan.CleanupError = types.StringValue(fmt.Sprintf("failed to get service account: %s", err))
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	// Delete all keys first (excluding system-managed keys)
	keysResp, err := iamSvc.Projects.ServiceAccounts.Keys.List(saName).
		KeyTypes("USER_MANAGED").Context(ctx).Do()
	var keysDeleted int64
	if err == nil {
		for _, key := range keysResp.Keys {
			if _, delErr := iamSvc.Projects.ServiceAccounts.Keys.Delete(key.Name).Context(ctx).Do(); delErr == nil {
				keysDeleted++
				tflog.Info(ctx, fmt.Sprintf("[SA Cleanup] Deleted key: %s", key.Name))
			}
		}
	}
	plan.KeysDeletedCount = types.Int64Value(keysDeleted)

	// Delete service account
	_, deleteErr := iamSvc.Projects.ServiceAccounts.Delete(saName).Context(ctx).Do()
	if deleteErr != nil {
		plan.CleanupStatus = types.StringValue("failed")
		plan.CleanupError = types.StringValue(fmt.Sprintf("failed to delete service account: %s", deleteErr))
	} else {
		plan.Deleted = types.BoolValue(true)
		plan.DeletionTimestamp = types.StringValue(time.Now().UTC().Format(time.RFC3339))
		plan.CleanupStatus = types.StringValue("deleted")
		tflog.Info(ctx, fmt.Sprintf("[SA Cleanup] Deleted service account: %s", saEmail))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LegacyCleanupServiceAccount) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state legacyCleanupServiceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Deleted.ValueBool() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	clientOptions := []option.ClientOption{}
	if serviceAccountKey := state.ServiceAccountKey.ValueString(); serviceAccountKey != "" {
		clientOption, err := newClientOptionFromEncodedServiceAccountKey(ctx, serviceAccountKey)
		if err != nil {
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}

		clientOptions = append(clientOptions, clientOption)
	}

	iamSvc, err := iam.NewService(ctx, clientOptions...)
	if err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	saName := fmt.Sprintf("projects/%s/serviceAccounts/%s", state.ProjectID.ValueString(), state.ServiceAccountEmail.ValueString())
	_, err = iamSvc.Projects.ServiceAccounts.Get(saName).Context(ctx).Do()
	if err != nil {
		if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == 404 {
			state.Deleted = types.BoolValue(true)
			state.CleanupStatus = types.StringValue("deleted")
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupServiceAccount) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state legacyCleanupServiceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	var plan legacyCleanupServiceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ServiceAccountKey = plan.ServiceAccountKey
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupServiceAccount) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op
}
