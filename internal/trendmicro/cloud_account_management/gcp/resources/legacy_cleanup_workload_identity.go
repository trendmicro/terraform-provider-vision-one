package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
)

var _ resource.Resource = &LegacyCleanupWorkloadIdentity{}

type LegacyCleanupWorkloadIdentity struct{}

type legacyCleanupWorkloadIdentityModel struct {
	ID                types.String `tfsdk:"id"`
	ProjectID         types.String `tfsdk:"project_id"`
	ServiceAccountKey types.String `tfsdk:"service_account_key"`
	Deleted           types.Bool   `tfsdk:"deleted"`
	PoolName          types.String `tfsdk:"pool_name"`
	ProviderName      types.String `tfsdk:"provider_name"`
	DeletionTimestamp types.String `tfsdk:"deletion_timestamp"`
	CleanupStatus     types.String `tfsdk:"cleanup_status"`
	CleanupError      types.String `tfsdk:"cleanup_error"`
}

func NewLegacyCleanupWorkloadIdentity() resource.Resource {
	return &LegacyCleanupWorkloadIdentity{}
}

func (r *LegacyCleanupWorkloadIdentity) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_LEGACY_CLEANUP_WORKLOAD_IDENTITY
}

func (r *LegacyCleanupWorkloadIdentity) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deletes the legacy Workload Identity Pool and its OIDC provider created by the Terraform Package Solution. Scans the project for pools whose ID contains `vision-one` and deletes the provider first, then the pool. Returns `cleanup_status = \"not_found\"` if no matching pool exists.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project ID containing the legacy Workload Identity Pool.",
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
				MarkdownDescription: "Whether the Workload Identity Pool was deleted.",
				Computed:            true,
			},
			"pool_name": schema.StringAttribute{
				MarkdownDescription: "Full resource name of the detected Workload Identity Pool.",
				Computed:            true,
			},
			"provider_name": schema.StringAttribute{
				MarkdownDescription: "Full resource name of the OIDC provider that was deleted.",
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

func (r *LegacyCleanupWorkloadIdentity) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan legacyCleanupWorkloadIdentityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	plan.ID = types.StringValue(projectID)
	plan.Deleted = types.BoolValue(false)
	plan.PoolName = types.StringValue("")
	plan.ProviderName = types.StringValue("")
	plan.DeletionTimestamp = types.StringValue("")
	plan.CleanupError = types.StringValue("")

	clientOptions := []option.ClientOption{}
	if serviceAccountKey := plan.ServiceAccountKey.ValueString(); serviceAccountKey != "" {
		clientOption, err := newClientOptionFromEncodedServiceAccountKey(ctx, serviceAccountKey)
		if err != nil {
			resp.Diagnostics.AddError("[WIF Cleanup] Invalid service account key", err.Error())
			return
		}

		clientOptions = append(clientOptions, clientOption)
	}

	iamSvc, err := iam.NewService(ctx, clientOptions...)
	if err != nil {
		resp.Diagnostics.AddError("[WIF Cleanup] Failed to create IAM client", err.Error())
		return
	}

	parent := fmt.Sprintf("projects/%s/locations/global", projectID)
	var legacyPool *iam.WorkloadIdentityPool

	if err := iamSvc.Projects.Locations.WorkloadIdentityPools.List(parent).Pages(ctx, func(page *iam.ListWorkloadIdentityPoolsResponse) error {
		for _, pool := range page.WorkloadIdentityPools {
			parts := strings.Split(pool.Name, "/")
			poolID := parts[len(parts)-1]
			if strings.Contains(poolID, "vision-one") ||
				strings.Contains(poolID, "visionone") ||
				strings.HasPrefix(poolID, "v1-workload-identity-pool-") {
				legacyPool = pool
				return errLegacyResourceFound
			}
		}
		return nil
	}); err != nil && !errors.Is(err, errLegacyResourceFound) {
		plan.CleanupStatus = types.StringValue("failed")
		plan.CleanupError = types.StringValue(fmt.Sprintf("failed to list workload identity pools: %s", err))
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	if legacyPool == nil {
		tflog.Info(ctx, fmt.Sprintf("[WIF Cleanup] No legacy workload identity pool found in project: %s", projectID))
		plan.CleanupStatus = types.StringValue("not_found")
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	plan.PoolName = types.StringValue(legacyPool.Name)

	// Delete OIDC providers first
	var deletedProviderName string
	_ = iamSvc.Projects.Locations.WorkloadIdentityPools.Providers.List(legacyPool.Name).Pages(ctx, func(page *iam.ListWorkloadIdentityPoolProvidersResponse) error {
		for _, provider := range page.WorkloadIdentityPoolProviders {
			_, delErr := iamSvc.Projects.Locations.WorkloadIdentityPools.Providers.Delete(provider.Name).Context(ctx).Do()
			if delErr == nil {
				deletedProviderName = provider.Name
				tflog.Info(ctx, fmt.Sprintf("[WIF Cleanup] Deleted OIDC provider: %s", provider.Name))
			}
		}
		return nil
	})
	if deletedProviderName != "" {
		plan.ProviderName = types.StringValue(deletedProviderName)
	}

	// Delete the pool
	_, deleteErr := iamSvc.Projects.Locations.WorkloadIdentityPools.Delete(legacyPool.Name).Context(ctx).Do()
	if deleteErr != nil {
		plan.CleanupStatus = types.StringValue("failed")
		plan.CleanupError = types.StringValue(fmt.Sprintf("failed to delete workload identity pool: %s", deleteErr))
	} else {
		plan.Deleted = types.BoolValue(true)
		plan.DeletionTimestamp = types.StringValue(time.Now().UTC().Format(time.RFC3339))
		plan.CleanupStatus = types.StringValue("deleted")
		tflog.Info(ctx, fmt.Sprintf("[WIF Cleanup] Deleted workload identity pool: %s", legacyPool.Name))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LegacyCleanupWorkloadIdentity) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state legacyCleanupWorkloadIdentityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Deleted.ValueBool() || state.PoolName.ValueString() == "" {
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

	pool, err := iamSvc.Projects.Locations.WorkloadIdentityPools.Get(state.PoolName.ValueString()).Context(ctx).Do()
	if err != nil || pool.State == "DELETED" {
		state.Deleted = types.BoolValue(true)
		state.CleanupStatus = types.StringValue("deleted")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupWorkloadIdentity) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state legacyCleanupWorkloadIdentityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	var plan legacyCleanupWorkloadIdentityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ServiceAccountKey = plan.ServiceAccountKey
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupWorkloadIdentity) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op
}
