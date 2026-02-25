package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/option"
)

var (
	_ resource.Resource                = &GCPTagKeyResource{}
	_ resource.ResourceWithConfigure   = &GCPTagKeyResource{}
	_ resource.ResourceWithImportState = &GCPTagKeyResource{}
)

func NewGCPTagKeyResource() resource.Resource {
	return &GCPTagKeyResource{}
}

type GCPTagKeyResource struct {
	client *api.CamClient
}

type gcpTagKeyResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ShortName      types.String `tfsdk:"short_name"`
	Parent         types.String `tfsdk:"parent"`
	Description    types.String `tfsdk:"description"`
	Name           types.String `tfsdk:"name"`
	NamespacedName types.String `tfsdk:"namespaced_name"`
	CreateTime     types.String `tfsdk:"create_time"`
	UpdateTime     types.String `tfsdk:"update_time"`
	Etag           types.String `tfsdk:"etag"`
}

func (r *GCPTagKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_TAG_KEY
}

func (r *GCPTagKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GCP resource tag key. Tag keys are used to organize GCP resources with labels that can be used in IAM policies and for resource organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Terraform resource identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"short_name": schema.StringAttribute{
				MarkdownDescription: "The short name of the tag key. This must be unique within the parent resource. " +
					"Must be 1-63 characters, beginning and ending with an alphanumeric character, " +
					"and containing only alphanumeric characters, underscores, and dashes.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parent": schema.StringAttribute{
				MarkdownDescription: "The resource name of the parent. Must be in the format 'projects/{project_id}.'",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the tag key. Maximum of 256 characters.",
				Optional:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The generated resource name of the tag key in the format 'tagKeys/{tag_key_id}'.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"namespaced_name": schema.StringAttribute{
				MarkdownDescription: "The namespaced name of the tag key.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"create_time": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the tag key was created.",
				Computed:            true,
			},
			"update_time": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the tag key was last updated.",
				Computed:            true,
			},
			"etag": schema.StringAttribute{
				MarkdownDescription: "Entity tag for concurrency control.",
				Computed:            true,
			},
		},
	}
}

func (r *GCPTagKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Configure]",
			fmt.Sprintf("Expected *trendmicro.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = &api.CamClient{
		Client: client,
	}
}

func (r *GCPTagKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gcpTagKeyResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get GCP credential
	gcpCred, err := api.GetGCPCredential(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Create]",
			fmt.Sprintf("Failed to get GCP credentials: %s", err.Error()),
		)
		return
	}

	// Create Cloud Resource Manager v3 client for tags
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(gcpCred))
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Create]",
			fmt.Sprintf("Failed to create Cloud Resource Manager client: %s", err.Error()),
		)
		return
	}

	// Build tag key request
	tagKey := &cloudresourcemanager.TagKey{
		ShortName:   plan.ShortName.ValueString(),
		Parent:      plan.Parent.ValueString(),
		Description: plan.Description.ValueString(),
	}

	tflog.Info(ctx, "Creating GCP tag key", map[string]interface{}{
		"short_name": plan.ShortName.ValueString(),
		"parent":     plan.Parent.ValueString(),
	})

	// Create tag key
	operation, err := crmService.TagKeys.Create(tagKey).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "ALREADY_EXISTS") {
			tflog.Info(ctx, "Tag key already exists, adopting existing resource", map[string]interface{}{
				"short_name": plan.ShortName.ValueString(),
				"parent":     plan.Parent.ValueString(),
			})
			existingTagKey, lookupErr := r.findExistingTagKey(ctx, crmService, plan.Parent.ValueString(), plan.ShortName.ValueString())
			if lookupErr != nil {
				resp.Diagnostics.AddError(
					"[GCP Tag Key][Create]",
					fmt.Sprintf("Tag key already exists but failed to look it up: %s", lookupErr.Error()),
				)
				return
			}
			updatedTagKey, updateErr := r.updateExistingTagKey(ctx, crmService, existingTagKey.Name, &plan)
			if updateErr != nil {
				resp.Diagnostics.AddError(
					"[GCP Tag Key][Create]",
					fmt.Sprintf("Tag key already exists but failed to update it: %s", updateErr.Error()),
				)
				return
			}
			r.adoptExistingTagKey(updatedTagKey, &plan)
			diags = resp.State.Set(ctx, &plan)
			resp.Diagnostics.Append(diags...)
			return
		}
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Create]",
			fmt.Sprintf("Error creating tag key '%s': %s", plan.ShortName.ValueString(), err.Error()),
		)
		return
	}

	// Wait for operation to complete
	finalOp, err := WaitForTagOperation(ctx, crmService, operation.Name)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Create]",
			fmt.Sprintf("Error waiting for tag key creation operation: %s", err.Error()),
		)
		return
	}

	if finalOp.Error != nil {
		if strings.Contains(finalOp.Error.Message, "ALREADY_EXISTS") {
			tflog.Info(ctx, "Tag key already exists (from operation), adopting existing resource", map[string]interface{}{
				"short_name": plan.ShortName.ValueString(),
				"parent":     plan.Parent.ValueString(),
			})
			existingTagKey, lookupErr := r.findExistingTagKey(ctx, crmService, plan.Parent.ValueString(), plan.ShortName.ValueString())
			if lookupErr != nil {
				resp.Diagnostics.AddError(
					"[GCP Tag Key][Create]",
					fmt.Sprintf("Tag key already exists but failed to look it up: %s", lookupErr.Error()),
				)
				return
			}
			updatedTagKey, updateErr := r.updateExistingTagKey(ctx, crmService, existingTagKey.Name, &plan)
			if updateErr != nil {
				resp.Diagnostics.AddError(
					"[GCP Tag Key][Create]",
					fmt.Sprintf("Tag key already exists but failed to update it: %s", updateErr.Error()),
				)
				return
			}
			r.adoptExistingTagKey(updatedTagKey, &plan)
			diags = resp.State.Set(ctx, &plan)
			resp.Diagnostics.Append(diags...)
			return
		}
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Create]",
			fmt.Sprintf("Tag key creation operation failed: %s", finalOp.Error.Message),
		)
		return
	}

	// Extract tag key name from operation metadata
	tagKeyName := ""
	if finalOp.Response != nil {
		// The response is a json.RawMessage containing the TagKey resource
		var responseData map[string]interface{}
		if unmarshalErr := json.Unmarshal(finalOp.Response, &responseData); unmarshalErr == nil {
			if nameVal, ok := responseData["name"]; ok {
				if strName, ok := nameVal.(string); ok {
					tagKeyName = strName
				}
			}
		}
	}

	if tagKeyName == "" {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Create]",
			"Failed to extract tag key name from operation response",
		)
		return
	}

	tflog.Debug(ctx, "Tag key created successfully", map[string]interface{}{
		"tag_key_name": tagKeyName,
	})

	// Read the created tag key to get all attributes
	createdTagKey, err := crmService.TagKeys.Get(tagKeyName).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Create]",
			fmt.Sprintf("Error reading created tag key '%s': %s", tagKeyName, err.Error()),
		)
		return
	}

	// Update state
	r.updateStateFromAPI(ctx, createdTagKey, &plan)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *GCPTagKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gcpTagKeyResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get GCP credential
	gcpCred, err := api.GetGCPCredential(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Read]",
			fmt.Sprintf("Failed to get GCP credentials: %s", err.Error()),
		)
		return
	}

	// Create Cloud Resource Manager v3 client
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(gcpCred))
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Read]",
			fmt.Sprintf("Failed to create Cloud Resource Manager client: %s", err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Reading GCP tag key", map[string]interface{}{
		"tag_key_name": state.Name.ValueString(),
	})

	// Read tag key
	tagKey, err := crmService.TagKeys.Get(state.Name.ValueString()).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "PERMISSION_DENIED") {
			// Tag key was deleted outside Terraform or is no longer accessible
			tflog.Info(ctx, "Tag key not found or not accessible, removing from state", map[string]interface{}{
				"tag_key_name": state.Name.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Read]",
			fmt.Sprintf("Error reading tag key '%s': %s", state.Name.ValueString(), err.Error()),
		)
		return
	}

	// Update state
	r.updateStateFromAPI(ctx, tagKey, &state)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *GCPTagKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gcpTagKeyResourceModel
	var state gcpTagKeyResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get GCP credential
	gcpCred, err := api.GetGCPCredential(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Update]",
			fmt.Sprintf("Failed to get GCP credentials: %s", err.Error()),
		)
		return
	}

	// Create Cloud Resource Manager v3 client
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(gcpCred))
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Update]",
			fmt.Sprintf("Failed to create Cloud Resource Manager client: %s", err.Error()),
		)
		return
	}

	// Build update request
	tagKey := &cloudresourcemanager.TagKey{
		Description: plan.Description.ValueString(),
	}

	// Determine update mask
	updateMask := []string{}
	if !plan.Description.Equal(state.Description) {
		updateMask = append(updateMask, "description")
	}

	if len(updateMask) == 0 {
		// No changes to make
		tflog.Debug(ctx, "No updates needed for tag key")
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	tflog.Info(ctx, "Updating GCP tag key", map[string]interface{}{
		"tag_key_name": state.Name.ValueString(),
		"update_mask":  updateMask,
	})

	// Update tag key
	patchCall := crmService.TagKeys.Patch(state.Name.ValueString(), tagKey)
	patchCall.UpdateMask(strings.Join(updateMask, ","))

	operation, err := patchCall.Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Update]",
			fmt.Sprintf("Error updating tag key '%s': %s", state.Name.ValueString(), err.Error()),
		)
		return
	}

	// Wait for operation to complete
	finalOp, err := WaitForTagOperation(ctx, crmService, operation.Name)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Update]",
			fmt.Sprintf("Error waiting for tag key update operation: %s", err.Error()),
		)
		return
	}

	if finalOp.Error != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Update]",
			fmt.Sprintf("Tag key update operation failed: %s", finalOp.Error.Message),
		)
		return
	}

	// Read updated tag key
	updatedTagKey, err := crmService.TagKeys.Get(state.Name.ValueString()).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Update]",
			fmt.Sprintf("Error reading updated tag key '%s': %s", state.Name.ValueString(), err.Error()),
		)
		return
	}

	// Update state
	r.updateStateFromAPI(ctx, updatedTagKey, &plan)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *GCPTagKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gcpTagKeyResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get GCP credential
	gcpCred, err := api.GetGCPCredential(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Delete]",
			fmt.Sprintf("Failed to get GCP credentials: %s", err.Error()),
		)
		return
	}

	// Create Cloud Resource Manager v3 client
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(gcpCred))
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Delete]",
			fmt.Sprintf("Failed to create Cloud Resource Manager client: %s", err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Deleting GCP tag key", map[string]interface{}{
		"tag_key_name": state.Name.ValueString(),
	})

	// Delete all child tag values first
	tagValuesResp, err := crmService.TagValues.List().Parent(state.Name.ValueString()).Context(ctx).Do()
	if err != nil {
		if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "not found") {
			resp.Diagnostics.AddError(
				"[GCP Tag Key][Delete]",
				fmt.Sprintf("Error listing child tag values for '%s': %s", state.Name.ValueString(), err.Error()),
			)
			return
		}
	} else {
		for _, tv := range tagValuesResp.TagValues {
			tflog.Info(ctx, "Deleting child tag value before tag key deletion", map[string]interface{}{
				"tag_value_name": tv.Name,
				"tag_key_name":   state.Name.ValueString(),
			})
			tvOp, tvErr := crmService.TagValues.Delete(tv.Name).Context(ctx).Do()
			if tvErr != nil {
				if strings.Contains(tvErr.Error(), "404") || strings.Contains(tvErr.Error(), "not found") {
					continue
				}
				resp.Diagnostics.AddError(
					"[GCP Tag Key][Delete]",
					fmt.Sprintf("Error deleting child tag value '%s': %s", tv.Name, tvErr.Error()),
				)
				return
			}
			finalTvOp, tvWaitErr := WaitForTagOperation(ctx, crmService, tvOp.Name)
			if tvWaitErr != nil {
				resp.Diagnostics.AddError(
					"[GCP Tag Key][Delete]",
					fmt.Sprintf("Error waiting for child tag value deletion '%s': %s", tv.Name, tvWaitErr.Error()),
				)
				return
			}
			if finalTvOp.Error != nil {
				resp.Diagnostics.AddError(
					"[GCP Tag Key][Delete]",
					fmt.Sprintf("Child tag value deletion failed '%s': %s", tv.Name, finalTvOp.Error.Message),
				)
				return
			}
		}
	}

	// Delete tag key
	operation, err := crmService.TagKeys.Delete(state.Name.ValueString()).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "PERMISSION_DENIED") {
			// Already deleted or no longer accessible
			tflog.Info(ctx, "Tag key already deleted or not accessible")
			return
		}
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Delete]",
			fmt.Sprintf("Error deleting tag key '%s': %s", state.Name.ValueString(), err.Error()),
		)
		return
	}

	// Wait for operation to complete
	finalOp, err := WaitForTagOperation(ctx, crmService, operation.Name)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Delete]",
			fmt.Sprintf("Error waiting for tag key deletion operation: %s", err.Error()),
		)
		return
	}

	if finalOp.Error != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Key][Delete]",
			fmt.Sprintf("Tag key deletion operation failed: %s", finalOp.Error.Message),
		)
		return
	}

	tflog.Info(ctx, "Tag key deleted successfully", map[string]interface{}{
		"tag_key_name": state.Name.ValueString(),
	})
}

func (r *GCPTagKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import using the tag key name (e.g., tagKeys/123456789)
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// findExistingTagKey looks up an existing tag key by parent and short_name.
func (r *GCPTagKeyResource) findExistingTagKey(ctx context.Context, crmService *cloudresourcemanager.Service, parent, shortName string) (*cloudresourcemanager.TagKey, error) {
	resp, err := crmService.TagKeys.List().Parent(parent).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list tag keys under %s: %w", parent, err)
	}
	for _, tk := range resp.TagKeys {
		if tk.ShortName == shortName {
			return tk, nil
		}
	}
	return nil, fmt.Errorf("tag key with short_name '%s' not found under parent '%s'", shortName, parent)
}

// adoptExistingTagKey sets only computed fields from an existing tag key,
// preserving all user-provided fields (short_name, parent, description).
func (r *GCPTagKeyResource) adoptExistingTagKey(tagKey *cloudresourcemanager.TagKey, model *gcpTagKeyResourceModel) {
	model.ID = types.StringValue(tagKey.Name)
	model.Name = types.StringValue(tagKey.Name)
	model.NamespacedName = types.StringValue(tagKey.NamespacedName)
	model.CreateTime = types.StringValue(tagKey.CreateTime)
	model.UpdateTime = types.StringValue(tagKey.UpdateTime)
	model.Etag = types.StringValue(tagKey.Etag)
}

// updateExistingTagKey patches the description of an existing tag key to match the plan.
func (r *GCPTagKeyResource) updateExistingTagKey(ctx context.Context, crmService *cloudresourcemanager.Service, tagKeyName string, plan *gcpTagKeyResourceModel) (*cloudresourcemanager.TagKey, error) {
	patchBody := &cloudresourcemanager.TagKey{
		Description: plan.Description.ValueString(),
	}
	operation, err := crmService.TagKeys.Patch(tagKeyName, patchBody).UpdateMask("description").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to update tag key description: %w", err)
	}
	finalOp, err := WaitForTagOperation(ctx, crmService, operation.Name)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for tag key update: %w", err)
	}
	if finalOp.Error != nil {
		return nil, fmt.Errorf("tag key update failed: %s", finalOp.Error.Message)
	}
	updated, err := crmService.TagKeys.Get(tagKeyName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to read updated tag key: %w", err)
	}
	return updated, nil
}

func (r *GCPTagKeyResource) updateStateFromAPI(_ context.Context, tagKey *cloudresourcemanager.TagKey, model *gcpTagKeyResourceModel) {
	model.ID = types.StringValue(tagKey.Name)
	model.Name = types.StringValue(tagKey.Name)
	model.ShortName = types.StringValue(tagKey.ShortName)
	// Don't update Parent from API response - preserve user's input value
	// The API may return a normalized form (e.g., project number instead of project ID)
	// which would cause Terraform inconsistency errors since Parent is immutable
	if model.Parent.IsNull() || model.Parent.IsUnknown() {
		model.Parent = types.StringValue(tagKey.Parent)
	}
	model.NamespacedName = types.StringValue(tagKey.NamespacedName)
	model.CreateTime = types.StringValue(tagKey.CreateTime)
	model.UpdateTime = types.StringValue(tagKey.UpdateTime)
	model.Etag = types.StringValue(tagKey.Etag)

	if tagKey.Description != "" {
		model.Description = types.StringValue(tagKey.Description)
	} else {
		model.Description = types.StringNull()
	}
}
