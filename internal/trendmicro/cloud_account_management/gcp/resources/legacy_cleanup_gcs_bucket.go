package resources

import (
	"context"
	"fmt"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	storagev1 "google.golang.org/api/storage/v1"
)

var _ resource.Resource = &LegacyCleanupGCSBucket{}

type LegacyCleanupGCSBucket struct{}

type legacyCleanupGCSBucketModel struct {
	ID                  types.String `tfsdk:"id"`
	ProjectID           types.String `tfsdk:"project_id"`
	ServiceAccountKey   types.String `tfsdk:"service_account_key"`
	PreserveStateBucket types.Bool   `tfsdk:"preserve_state_bucket"`
	ForceDeleteBucket   types.Bool   `tfsdk:"force_delete_bucket"`
	DestinationBucket   types.String `tfsdk:"destination_bucket"`
	Deleted             types.Bool   `tfsdk:"deleted"`
	Archived            types.Bool   `tfsdk:"archived"`
	BucketName          types.String `tfsdk:"bucket_name"`
	StateFileExists     types.Bool   `tfsdk:"state_file_exists"`
	StateCopied         types.Bool   `tfsdk:"state_copied"`
	DeletionTimestamp   types.String `tfsdk:"deletion_timestamp"`
	CleanupStatus       types.String `tfsdk:"cleanup_status"`
	CleanupError        types.String `tfsdk:"cleanup_error"`
}

func NewLegacyCleanupGCSBucket() resource.Resource {
	return &LegacyCleanupGCSBucket{}
}

func (r *LegacyCleanupGCSBucket) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_LEGACY_CLEANUP_GCS_BUCKET
}

func (r *LegacyCleanupGCSBucket) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Cleans up the legacy GCS Terraform state bucket (`trendmicro-v1-{project_id}`) created by the Terraform Package Solution. Supports archive mode (labeling instead of deletion) to preserve the state file. Returns `cleanup_status = \"not_found\"` if no legacy bucket exists.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project ID containing the legacy state bucket.",
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
			"preserve_state_bucket": schema.BoolAttribute{
				MarkdownDescription: "If true (default), add labels to archive the bucket instead of deleting it.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"force_delete_bucket": schema.BoolAttribute{
				MarkdownDescription: "If true, delete the legacy source bucket after the state file has been copied to destination_bucket. Overrides preserve_state_bucket. Default: false.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"destination_bucket": schema.StringAttribute{
				MarkdownDescription: "Optional destination GCS bucket that receives a copy of the state file renamed as `{project_id}.tfstate` before the legacy bucket is archived or deleted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"deleted":  schema.BoolAttribute{Computed: true},
			"archived": schema.BoolAttribute{Computed: true},
			"bucket_name": schema.StringAttribute{
				MarkdownDescription: "The name of the legacy bucket that was detected.",
				Computed:            true,
			},
			"state_file_exists": schema.BoolAttribute{
				MarkdownDescription: "Whether default.tfstate was found in the bucket.",
				Computed:            true,
			},
			"state_copied": schema.BoolAttribute{
				MarkdownDescription: "Whether copying the state file to destination_bucket as `{project_id}.tfstate` succeeded.",
				Computed:            true,
			},
			"deletion_timestamp": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the cleanup was performed.",
				Computed:            true,
			},
			"cleanup_status": schema.StringAttribute{
				MarkdownDescription: "Status: deleted, archived, not_found, or failed.",
				Computed:            true,
			},
			"cleanup_error": schema.StringAttribute{
				MarkdownDescription: "Error message if cleanup failed.",
				Computed:            true,
			},
		},
	}
}

func (r *LegacyCleanupGCSBucket) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan legacyCleanupGCSBucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	bucketName := config.LEGACY_GCP_GCS_BUCKET_PREFIX + projectID

	if plan.DestinationBucket.IsNull() || plan.DestinationBucket.IsUnknown() {
		plan.DestinationBucket = types.StringValue("")
	}

	plan.ID = types.StringValue(projectID)
	plan.BucketName = types.StringValue(bucketName)
	plan.Deleted = types.BoolValue(false)
	plan.Archived = types.BoolValue(false)
	plan.StateFileExists = types.BoolValue(false)
	plan.StateCopied = types.BoolValue(false)
	plan.DeletionTimestamp = types.StringValue("")
	plan.CleanupError = types.StringValue("")

	clientOptions := []option.ClientOption{}
	if serviceAccountKey := plan.ServiceAccountKey.ValueString(); serviceAccountKey != "" {
		clientOption, err := newClientOptionFromEncodedServiceAccountKey(ctx, serviceAccountKey)
		if err != nil {
			resp.Diagnostics.AddError("[GCS Bucket Cleanup] Invalid service account key", err.Error())
			return
		}

		clientOptions = append(clientOptions, clientOption)
	}

	storageSvc, err := storagev1.NewService(ctx, clientOptions...)
	if err != nil {
		resp.Diagnostics.AddError("[GCS Bucket Cleanup] Failed to create GCS client", err.Error())
		return
	}

	// Check if bucket exists
	_, err = storageSvc.Buckets.Get(bucketName).Context(ctx).Do()
	if err != nil {
		if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == 404 {
			tflog.Info(ctx, fmt.Sprintf("[GCS Bucket Cleanup] Bucket not found: %s", bucketName))
			plan.CleanupStatus = types.StringValue("not_found")
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			return
		}
		plan.CleanupStatus = types.StringValue("failed")
		plan.CleanupError = types.StringValue(fmt.Sprintf("failed to check bucket: %s", err))
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	// Check for state file
	stateFileExists := false
	objs, listErr := storageSvc.Objects.List(bucketName).Prefix(config.LEGACY_GCP_STATE_FILE_NAME).MaxResults(1).Context(ctx).Do()
	if listErr == nil && len(objs.Items) > 0 {
		stateFileExists = true
	}
	plan.StateFileExists = types.BoolValue(stateFileExists)

	renamedObject := projectID + ".tfstate"

	destinationBucket := plan.DestinationBucket.ValueString()
	if destinationBucket != "" {
		_, copyErr := storageSvc.Objects.Copy(
			bucketName,
			config.LEGACY_GCP_STATE_FILE_NAME,
			destinationBucket,
			renamedObject,
			nil,
		).Context(ctx).Do()
		if copyErr != nil {
			tflog.Warn(ctx, fmt.Sprintf(
				"[GCS Bucket Cleanup] Failed to copy state file from %s/%s to %s/%s: %s",
				bucketName, config.LEGACY_GCP_STATE_FILE_NAME,
				destinationBucket, renamedObject,
				copyErr,
			))
		} else {
			plan.StateCopied = types.BoolValue(true)
			tflog.Info(ctx, fmt.Sprintf(
				"[GCS Bucket Cleanup] Copied state file from %s/%s to %s/%s",
				bucketName, config.LEGACY_GCP_STATE_FILE_NAME,
				destinationBucket, renamedObject,
			))
			// Rename default.tfstate → {project-id}.tfstate in source bucket to avoid name collisions
			_, renameErr := storageSvc.Objects.Copy(
				bucketName, config.LEGACY_GCP_STATE_FILE_NAME,
				bucketName, renamedObject,
				nil,
			).Context(ctx).Do()
			if renameErr != nil {
				tflog.Warn(ctx, fmt.Sprintf(
					"[GCS Bucket Cleanup] Failed to rename %s/%s to %s/%s: %s",
					bucketName, config.LEGACY_GCP_STATE_FILE_NAME,
					bucketName, renamedObject,
					renameErr,
				))
			} else {
				_ = storageSvc.Objects.Delete(bucketName, config.LEGACY_GCP_STATE_FILE_NAME).Context(ctx).Do()
				tflog.Info(ctx, fmt.Sprintf(
					"[GCS Bucket Cleanup] Renamed %s/%s to %s/%s",
					bucketName, config.LEGACY_GCP_STATE_FILE_NAME,
					bucketName, renamedObject,
				))
			}
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if plan.ForceDeleteBucket.ValueBool() {
		deleteAllBucketObjects(ctx, storageSvc, bucketName)
		deleteErr := storageSvc.Buckets.Delete(bucketName).Context(ctx).Do()
		if deleteErr != nil {
			plan.CleanupStatus = types.StringValue("failed")
			plan.CleanupError = types.StringValue(fmt.Sprintf("failed to delete bucket: %s", deleteErr))
			resp.Diagnostics.AddError("[GCS Bucket Cleanup] Failed to delete bucket", fmt.Sprintf("failed to delete bucket %s: %s", bucketName, deleteErr))
		} else {
			plan.Deleted = types.BoolValue(true)
			plan.DeletionTimestamp = types.StringValue(now)
			plan.CleanupStatus = types.StringValue("deleted")
			tflog.Info(ctx, fmt.Sprintf("[GCS Bucket Cleanup] Deleted bucket: %s", bucketName))
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	if plan.PreserveStateBucket.ValueBool() {
		// Archive: patch bucket labels.
		// GCS label values only allow lowercase letters, digits, hyphens, underscores.
		// RFC3339 contains "T", ":", "Z" which are invalid — use a safe format instead.
		labelTimestamp := time.Now().UTC().Format("2006-01-02-150405z")
		patch := &storagev1.Bucket{
			Labels: map[string]string{
				"archived-by": "terraform-cleanup",
				"archived-at": labelTimestamp,
			},
		}
		_, updateErr := storageSvc.Buckets.Patch(bucketName, patch).Context(ctx).Do()
		if updateErr != nil {
			plan.CleanupStatus = types.StringValue("failed")
			plan.CleanupError = types.StringValue(fmt.Sprintf("failed to archive bucket: %s", updateErr))
		} else {
			plan.Archived = types.BoolValue(true)
			plan.DeletionTimestamp = types.StringValue(now)
			plan.CleanupStatus = types.StringValue("archived")
			tflog.Info(ctx, fmt.Sprintf("[GCS Bucket Cleanup] Archived bucket: %s", bucketName))
		}
	} else {
		if stateFileExists {
			plan.CleanupStatus = types.StringValue("failed")
			plan.CleanupError = types.StringValue("state file exists; set force_delete_bucket=true to delete after copying the state file, or preserve_state_bucket=true to archive")
		} else {
			deleteAllBucketObjects(ctx, storageSvc, bucketName)
			deleteErr := storageSvc.Buckets.Delete(bucketName).Context(ctx).Do()
			if deleteErr != nil {
				plan.CleanupStatus = types.StringValue("failed")
				plan.CleanupError = types.StringValue(fmt.Sprintf("failed to delete bucket: %s", deleteErr))
				resp.Diagnostics.AddError("[GCS Bucket Cleanup] Failed to delete bucket", fmt.Sprintf("failed to delete bucket %s: %s", bucketName, deleteErr))
			} else {
				plan.Deleted = types.BoolValue(true)
				plan.DeletionTimestamp = types.StringValue(now)
				plan.CleanupStatus = types.StringValue("deleted")
				tflog.Info(ctx, fmt.Sprintf("[GCS Bucket Cleanup] Deleted bucket: %s", bucketName))
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LegacyCleanupGCSBucket) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state legacyCleanupGCSBucketModel
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

	storageSvc, err := storagev1.NewService(ctx, clientOptions...)
	if err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	_, err = storageSvc.Buckets.Get(state.BucketName.ValueString()).Context(ctx).Do()
	if err != nil {
		if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == 404 {
			state.Deleted = types.BoolValue(true)
			state.CleanupStatus = types.StringValue("deleted")
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupGCSBucket) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state legacyCleanupGCSBucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	var plan legacyCleanupGCSBucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ServiceAccountKey = plan.ServiceAccountKey
	state.PreserveStateBucket = plan.PreserveStateBucket
	state.ForceDeleteBucket = plan.ForceDeleteBucket
	state.DestinationBucket = plan.DestinationBucket
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupGCSBucket) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op: cleanup was done during Create; terraform destroy removes state only.
}

// deleteAllBucketObjects deletes all objects and all versioned objects from a GCS bucket.
func deleteAllBucketObjects(ctx context.Context, storageSvc *storagev1.Service, bucketName string) {
	var pageToken string
	for {
		listReq := storageSvc.Objects.List(bucketName).Versions(true)
		if pageToken != "" {
			listReq = listReq.PageToken(pageToken)
		}
		objList, err := listReq.Context(ctx).Do()
		if err != nil {
			break
		}
		for _, obj := range objList.Items {
			_ = storageSvc.Objects.Delete(bucketName, obj.Name).Generation(obj.Generation).Context(ctx).Do()
		}
		if objList.NextPageToken == "" {
			break
		}
		pageToken = objList.NextPageToken
	}
}
