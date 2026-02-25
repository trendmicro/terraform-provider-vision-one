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
	_ resource.Resource                = &GCPTagValueResource{}
	_ resource.ResourceWithConfigure   = &GCPTagValueResource{}
	_ resource.ResourceWithImportState = &GCPTagValueResource{}
)

func NewGCPTagValueResource() resource.Resource {
	return &GCPTagValueResource{}
}

type GCPTagValueResource struct {
	client *api.CamClient
}

type gcpTagValueResourceModel struct {
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

func (r *GCPTagValueResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_TAG_VALUE
}

func (r *GCPTagValueResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GCP resource tag value. Tag values are the allowed values for a specific tag key and can be attached to GCP resources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Terraform resource identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"short_name": schema.StringAttribute{
				MarkdownDescription: "The short name of the tag value. This must be unique within the parent tag key. " +
					"Must be 1-63 characters, beginning and ending with an alphanumeric character, " +
					"and containing only alphanumeric characters, underscores, and dashes.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parent": schema.StringAttribute{
				MarkdownDescription: "The resource name of the parent tag key. Must be in the format 'tagKeys/{tag_key_id}'.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the tag value. Maximum of 256 characters.",
				Optional:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The generated resource name of the tag value in the format 'tagValues/{tag_value_id}'.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"namespaced_name": schema.StringAttribute{
				MarkdownDescription: "The namespaced name of the tag value.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"create_time": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the tag value was created.",
				Computed:            true,
			},
			"update_time": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the tag value was last updated.",
				Computed:            true,
			},
			"etag": schema.StringAttribute{
				MarkdownDescription: "Entity tag for concurrency control.",
				Computed:            true,
			},
		},
	}
}

func (r *GCPTagValueResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Configure]",
			fmt.Sprintf("Expected *trendmicro.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = &api.CamClient{
		Client: client,
	}
}

func (r *GCPTagValueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gcpTagValueResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get GCP credential
	gcpCred, err := api.GetGCPCredential(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Create]",
			fmt.Sprintf("Failed to get GCP credentials: %s", err.Error()),
		)
		return
	}

	// Create Cloud Resource Manager v3 client for tags
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(gcpCred))
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Create]",
			fmt.Sprintf("Failed to create Cloud Resource Manager client: %s", err.Error()),
		)
		return
	}

	// Build tag value request
	tagValue := &cloudresourcemanager.TagValue{
		ShortName:   plan.ShortName.ValueString(),
		Parent:      plan.Parent.ValueString(),
		Description: plan.Description.ValueString(),
	}

	tflog.Info(ctx, "Creating GCP tag value", map[string]interface{}{
		"short_name": plan.ShortName.ValueString(),
		"parent":     plan.Parent.ValueString(),
	})

	// Create tag value
	operation, err := crmService.TagValues.Create(tagValue).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "ALREADY_EXISTS") {
			tflog.Info(ctx, "Tag value already exists, adopting existing resource", map[string]interface{}{
				"short_name": plan.ShortName.ValueString(),
				"parent":     plan.Parent.ValueString(),
			})
			existingTagValue, lookupErr := r.findExistingTagValue(ctx, crmService, plan.Parent.ValueString(), plan.ShortName.ValueString())
			if lookupErr != nil {
				resp.Diagnostics.AddError(
					"[GCP Tag Value][Create]",
					fmt.Sprintf("Tag value already exists but failed to look it up: %s", lookupErr.Error()),
				)
				return
			}
			updatedTagValue, updateErr := r.updateExistingTagValue(ctx, crmService, existingTagValue.Name, &plan)
			if updateErr != nil {
				resp.Diagnostics.AddError(
					"[GCP Tag Value][Create]",
					fmt.Sprintf("Tag value already exists but failed to update it: %s", updateErr.Error()),
				)
				return
			}
			r.adoptExistingTagValue(updatedTagValue, &plan)
			diags = resp.State.Set(ctx, &plan)
			resp.Diagnostics.Append(diags...)
			return
		}
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Create]",
			fmt.Sprintf("Error creating tag value '%s': %s", plan.ShortName.ValueString(), err.Error()),
		)
		return
	}

	// Wait for operation to complete
	finalOp, err := WaitForTagOperation(ctx, crmService, operation.Name)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Create]",
			fmt.Sprintf("Error waiting for tag value creation operation: %s", err.Error()),
		)
		return
	}

	if finalOp.Error != nil {
		if strings.Contains(finalOp.Error.Message, "ALREADY_EXISTS") {
			tflog.Info(ctx, "Tag value already exists (from operation), adopting existing resource", map[string]interface{}{
				"short_name": plan.ShortName.ValueString(),
				"parent":     plan.Parent.ValueString(),
			})
			existingTagValue, lookupErr := r.findExistingTagValue(ctx, crmService, plan.Parent.ValueString(), plan.ShortName.ValueString())
			if lookupErr != nil {
				resp.Diagnostics.AddError(
					"[GCP Tag Value][Create]",
					fmt.Sprintf("Tag value already exists but failed to look it up: %s", lookupErr.Error()),
				)
				return
			}
			updatedTagValue, updateErr := r.updateExistingTagValue(ctx, crmService, existingTagValue.Name, &plan)
			if updateErr != nil {
				resp.Diagnostics.AddError(
					"[GCP Tag Value][Create]",
					fmt.Sprintf("Tag value already exists but failed to update it: %s", updateErr.Error()),
				)
				return
			}
			r.adoptExistingTagValue(updatedTagValue, &plan)
			diags = resp.State.Set(ctx, &plan)
			resp.Diagnostics.Append(diags...)
			return
		}
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Create]",
			fmt.Sprintf("Tag value creation operation failed: %s", finalOp.Error.Message),
		)
		return
	}

	// Extract tag value name from operation metadata
	tagValueName := ""
	if finalOp.Response != nil {
		// The response is a json.RawMessage containing the TagValue resource
		var responseData map[string]interface{}
		if unmarshalErr := json.Unmarshal(finalOp.Response, &responseData); unmarshalErr == nil {
			if nameVal, ok := responseData["name"]; ok {
				if strName, ok := nameVal.(string); ok {
					tagValueName = strName
				}
			}
		}
	}

	if tagValueName == "" {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Create]",
			"Failed to extract tag value name from operation response",
		)
		return
	}

	tflog.Debug(ctx, "Tag value created successfully", map[string]interface{}{
		"tag_value_name": tagValueName,
	})

	// Read the created tag value to get all attributes
	createdTagValue, err := crmService.TagValues.Get(tagValueName).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Create]",
			fmt.Sprintf("Error reading created tag value '%s': %s", tagValueName, err.Error()),
		)
		return
	}

	// Update state
	r.updateStateFromAPI(ctx, createdTagValue, &plan)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *GCPTagValueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gcpTagValueResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get GCP credential
	gcpCred, err := api.GetGCPCredential(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Read]",
			fmt.Sprintf("Failed to get GCP credentials: %s", err.Error()),
		)
		return
	}

	// Create Cloud Resource Manager v3 client
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(gcpCred))
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Read]",
			fmt.Sprintf("Failed to create Cloud Resource Manager client: %s", err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Reading GCP tag value", map[string]interface{}{
		"tag_value_name": state.Name.ValueString(),
	})

	// Read tag value
	tagValue, err := crmService.TagValues.Get(state.Name.ValueString()).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "PERMISSION_DENIED") {
			// Tag value was deleted outside Terraform or is no longer accessible
			tflog.Info(ctx, "Tag value not found or not accessible, removing from state", map[string]interface{}{
				"tag_value_name": state.Name.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Read]",
			fmt.Sprintf("Error reading tag value '%s': %s", state.Name.ValueString(), err.Error()),
		)
		return
	}

	// Update state
	r.updateStateFromAPI(ctx, tagValue, &state)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *GCPTagValueResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gcpTagValueResourceModel
	var state gcpTagValueResourceModel

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
			"[GCP Tag Value][Update]",
			fmt.Sprintf("Failed to get GCP credentials: %s", err.Error()),
		)
		return
	}

	// Create Cloud Resource Manager v3 client
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(gcpCred))
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Update]",
			fmt.Sprintf("Failed to create Cloud Resource Manager client: %s", err.Error()),
		)
		return
	}

	// Build update request
	tagValue := &cloudresourcemanager.TagValue{
		Description: plan.Description.ValueString(),
	}

	// Determine update mask
	updateMask := []string{}
	if !plan.Description.Equal(state.Description) {
		updateMask = append(updateMask, "description")
	}

	if len(updateMask) == 0 {
		// No changes to make
		tflog.Debug(ctx, "No updates needed for tag value")
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	tflog.Info(ctx, "Updating GCP tag value", map[string]interface{}{
		"tag_value_name": state.Name.ValueString(),
		"update_mask":    updateMask,
	})

	// Update tag value
	patchCall := crmService.TagValues.Patch(state.Name.ValueString(), tagValue)
	patchCall.UpdateMask(strings.Join(updateMask, ","))

	operation, err := patchCall.Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Update]",
			fmt.Sprintf("Error updating tag value '%s': %s", state.Name.ValueString(), err.Error()),
		)
		return
	}

	// Wait for operation to complete
	finalOp, err := WaitForTagOperation(ctx, crmService, operation.Name)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Update]",
			fmt.Sprintf("Error waiting for tag value update operation: %s", err.Error()),
		)
		return
	}

	if finalOp.Error != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Update]",
			fmt.Sprintf("Tag value update operation failed: %s", finalOp.Error.Message),
		)
		return
	}

	// Read updated tag value
	updatedTagValue, err := crmService.TagValues.Get(state.Name.ValueString()).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Update]",
			fmt.Sprintf("Error reading updated tag value '%s': %s", state.Name.ValueString(), err.Error()),
		)
		return
	}

	// Update state
	r.updateStateFromAPI(ctx, updatedTagValue, &plan)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *GCPTagValueResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gcpTagValueResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get GCP credential
	gcpCred, err := api.GetGCPCredential(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Delete]",
			fmt.Sprintf("Failed to get GCP credentials: %s", err.Error()),
		)
		return
	}

	// Create Cloud Resource Manager v3 client
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(gcpCred))
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Delete]",
			fmt.Sprintf("Failed to create Cloud Resource Manager client: %s", err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Deleting GCP tag value", map[string]interface{}{
		"tag_value_name": state.Name.ValueString(),
	})

	// Delete tag value
	operation, err := crmService.TagValues.Delete(state.Name.ValueString()).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "PERMISSION_DENIED") {
			// Already deleted or no longer accessible
			tflog.Info(ctx, "Tag value already deleted or not accessible")
			return
		}
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Delete]",
			fmt.Sprintf("Error deleting tag value '%s': %s", state.Name.ValueString(), err.Error()),
		)
		return
	}

	// Wait for operation to complete
	finalOp, err := WaitForTagOperation(ctx, crmService, operation.Name)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Delete]",
			fmt.Sprintf("Error waiting for tag value deletion operation: %s", err.Error()),
		)
		return
	}

	if finalOp.Error != nil {
		resp.Diagnostics.AddError(
			"[GCP Tag Value][Delete]",
			fmt.Sprintf("Tag value deletion operation failed: %s", finalOp.Error.Message),
		)
		return
	}

	tflog.Info(ctx, "Tag value deleted successfully", map[string]interface{}{
		"tag_value_name": state.Name.ValueString(),
	})
}

func (r *GCPTagValueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import using the tag value name (e.g., tagValues/123456789)
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// findExistingTagValue looks up an existing tag value by parent and short_name.
func (r *GCPTagValueResource) findExistingTagValue(ctx context.Context, crmService *cloudresourcemanager.Service, parent, shortName string) (*cloudresourcemanager.TagValue, error) {
	resp, err := crmService.TagValues.List().Parent(parent).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list tag values under %s: %w", parent, err)
	}
	for _, tv := range resp.TagValues {
		if tv.ShortName == shortName {
			return tv, nil
		}
	}
	return nil, fmt.Errorf("tag value with short_name '%s' not found under parent '%s'", shortName, parent)
}

// adoptExistingTagValue sets only computed fields from an existing tag value,
// preserving all user-provided fields (short_name, parent, description).
func (r *GCPTagValueResource) adoptExistingTagValue(tagValue *cloudresourcemanager.TagValue, model *gcpTagValueResourceModel) {
	model.ID = types.StringValue(tagValue.Name)
	model.Name = types.StringValue(tagValue.Name)
	model.NamespacedName = types.StringValue(tagValue.NamespacedName)
	model.CreateTime = types.StringValue(tagValue.CreateTime)
	model.UpdateTime = types.StringValue(tagValue.UpdateTime)
	model.Etag = types.StringValue(tagValue.Etag)
}

// updateExistingTagValue patches the description of an existing tag value to match the plan.
func (r *GCPTagValueResource) updateExistingTagValue(ctx context.Context, crmService *cloudresourcemanager.Service, tagValueName string, plan *gcpTagValueResourceModel) (*cloudresourcemanager.TagValue, error) {
	patchBody := &cloudresourcemanager.TagValue{
		Description: plan.Description.ValueString(),
	}
	operation, err := crmService.TagValues.Patch(tagValueName, patchBody).UpdateMask("description").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to update tag value description: %w", err)
	}
	finalOp, err := WaitForTagOperation(ctx, crmService, operation.Name)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for tag value update: %w", err)
	}
	if finalOp.Error != nil {
		return nil, fmt.Errorf("tag value update failed: %s", finalOp.Error.Message)
	}
	updated, err := crmService.TagValues.Get(tagValueName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to read updated tag value: %w", err)
	}
	return updated, nil
}

func (r *GCPTagValueResource) updateStateFromAPI(_ context.Context, tagValue *cloudresourcemanager.TagValue, model *gcpTagValueResourceModel) {
	model.ID = types.StringValue(tagValue.Name)
	model.Name = types.StringValue(tagValue.Name)
	model.ShortName = types.StringValue(tagValue.ShortName)
	// Don't update Parent from API response - preserve user's input value
	// The API may return a normalized form which would cause Terraform inconsistency errors
	// since Parent is immutable
	if model.Parent.IsNull() || model.Parent.IsUnknown() {
		model.Parent = types.StringValue(tagValue.Parent)
	}
	model.NamespacedName = types.StringValue(tagValue.NamespacedName)
	model.CreateTime = types.StringValue(tagValue.CreateTime)
	model.UpdateTime = types.StringValue(tagValue.UpdateTime)
	model.Etag = types.StringValue(tagValue.Etag)

	if tagValue.Description != "" {
		model.Description = types.StringValue(tagValue.Description)
	} else {
		model.Description = types.StringNull()
	}
}
