package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/option"
	"google.golang.org/api/serviceusage/v1"
)

type EnableAPIServices struct {
	client *api.CamClient
}

type enableAPIServicesResourceModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	Services  types.List   `tfsdk:"services"`
}

func NewEnableAPIServices() resource.Resource {
	return &EnableAPIServices{
		client: &api.CamClient{},
	}
}

func (r *EnableAPIServices) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_ENABLE_API_SERVICES
}

func (r *EnableAPIServices) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Enables required GCP API services for Trend Micro Vision One Cloud Account Management. " +
			"This resource ensures that all necessary APIs are enabled in the specified GCP project. " +
			"Please note that API services are not disabled when this resource is destroyed to prevent disruption to other resources.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project ID where API services will be enabled. If not specified, uses the project from provider configuration or default GCP credentials.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"services": schema.ListAttribute{
				ElementType: types.StringType,
				MarkdownDescription: "List of GCP API service names to enable (e.g., `iamcredentials.googleapis.com`). " +
					"If not specified, defaults to the required services for Vision One CAM. " +
					"Please note that this configuration can be extended when new features require additional API services.",
				Optional: true,
				Computed: true,
				Default:  listdefault.StaticValue(cam.ConvertStringSliceToListValue(config.GCP_REQUIRED_ENABLE_API_AND_SERVICE)),
			},
		},
	}
}

func (r *EnableAPIServices) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"[Enable API Services][Configure]",
			fmt.Sprintf("Expected *trendmicro.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client.Client = client
}

func (r *EnableAPIServices) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan enableAPIServicesResourceModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Get project ID
	projectID := plan.ProjectID.ValueString()
	gcpClients, diags := api.GetGCPClients(ctx, projectID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set the resolved project ID in state
	plan.ProjectID = types.StringValue(gcpClients.ProjectID)

	// Get services list
	var services []string
	if diagsList := plan.Services.ElementsAs(ctx, &services, false); diagsList.HasError() {
		resp.Diagnostics.Append(diagsList...)
		return
	}

	tflog.Info(ctx, "Enabling GCP API services", map[string]interface{}{
		"project_id": gcpClients.ProjectID,
		"services":   services,
	})

	// Create Service Usage client
	serviceUsageClient, err := serviceusage.NewService(ctx, option.WithCredentials(gcpClients.Credential))
	if err != nil {
		resp.Diagnostics.AddError(
			"[Enable API Services][Create]",
			fmt.Sprintf("Failed to create Service Usage client: %s", err),
		)
		return
	}

	// Enable each service concurrently
	var (
		mu              sync.Mutex
		wg              sync.WaitGroup
		billingWarnings []string
		enableErrors    []string
	)

	sem := cam.GCPServiceUsageSem
	for _, service := range services {
		wg.Add(1)
		go func(svc string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire slot
			defer func() { <-sem }() // release slot
			if err := r.enableService(ctx, serviceUsageClient, gcpClients.ProjectID, svc); err != nil {
				var be *billingError
				if errors.As(err, &be) {
					tflog.Warn(ctx, "Skipping service enable due to missing billing account", map[string]interface{}{
						"service":    svc,
						"project_id": gcpClients.ProjectID,
					})
					mu.Lock()
					billingWarnings = append(billingWarnings, svc)
					mu.Unlock()
					return
				}
				mu.Lock()
				enableErrors = append(enableErrors, fmt.Sprintf("Failed to enable service %s in project %s: %s", svc, gcpClients.ProjectID, err))
				mu.Unlock()
				return
			}
			tflog.Debug(ctx, "Successfully enabled service", map[string]interface{}{
				"service": svc,
			})
		}(service)
	}
	wg.Wait()

	if len(enableErrors) > 0 {
		resp.Diagnostics.AddError(
			"[Enable API Services][Create]",
			strings.Join(enableErrors, "; "),
		)
		return
	}

	if len(billingWarnings) > 0 {
		resp.Diagnostics.AddWarning(
			"[Enable API Services][Create] Billing not enabled",
			fmt.Sprintf("Project '%s' does not have a billing account linked. "+
				"The following services could not be enabled: %s. "+
				"Please link a billing account to the project and re-run, or enable these services manually in the GCP Console.",
				gcpClients.ProjectID, strings.Join(billingWarnings, ", ")),
		)
	}

	tflog.Info(ctx, "Successfully enabled all API services", map[string]interface{}{
		"project_id": gcpClients.ProjectID,
		"count":      len(services),
	})

	// Save state
	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
}

func (r *EnableAPIServices) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state enableAPIServicesResourceModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Get project ID
	projectID := state.ProjectID.ValueString()
	gcpClients, diags := api.GetGCPClients(ctx, projectID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Get services list
	var services []string
	if diagsList := state.Services.ElementsAs(ctx, &services, false); diagsList.HasError() {
		resp.Diagnostics.Append(diagsList...)
		return
	}

	tflog.Debug(ctx, "Reading GCP API services state", map[string]interface{}{
		"project_id": gcpClients.ProjectID,
		"services":   services,
	})

	// Create Service Usage client
	serviceUsageClient, err := serviceusage.NewService(ctx, option.WithCredentials(gcpClients.Credential))
	if err != nil {
		resp.Diagnostics.AddError(
			"[Enable API Services][Read]",
			fmt.Sprintf("Failed to create Service Usage client: %s", err),
		)
		return
	}

	// Check if services are still enabled concurrently
	type serviceStatus struct {
		service string
		enabled bool
		err     error
	}

	statusResults := make([]serviceStatus, len(services))
	var wg sync.WaitGroup
	sem := cam.GCPServiceUsageSem
	for i, service := range services {
		wg.Add(1)
		go func(idx int, svc string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire slot
			defer func() { <-sem }() // release slot
			enabled, err := r.isServiceEnabled(ctx, serviceUsageClient, gcpClients.ProjectID, svc)
			statusResults[idx] = serviceStatus{service: svc, enabled: enabled, err: err}
		}(i, service)
	}
	wg.Wait()

	allEnabled := true
	for _, result := range statusResults {
		if result.err != nil {
			tflog.Warn(ctx, "Failed to check service status", map[string]interface{}{
				"service": result.service,
				"error":   result.err.Error(),
			})
			continue
		}
		if !result.enabled {
			allEnabled = false
			tflog.Warn(ctx, "Service is not enabled", map[string]interface{}{
				"service": result.service,
			})
		}
	}

	if !allEnabled {
		tflog.Info(ctx, "Some services are not enabled, resource may need recreation")
	}

	// Save state (keep existing state even if some services are disabled)
	if diags := resp.State.Set(ctx, state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
}

func (r *EnableAPIServices) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan enableAPIServicesResourceModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Get project ID
	projectID := plan.ProjectID.ValueString()
	gcpClients, diags := api.GetGCPClients(ctx, projectID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Get services list
	var services []string
	if diagsList := plan.Services.ElementsAs(ctx, &services, false); diagsList.HasError() {
		resp.Diagnostics.Append(diagsList...)
		return
	}

	tflog.Info(ctx, "Updating GCP API services", map[string]interface{}{
		"project_id": gcpClients.ProjectID,
		"services":   services,
	})

	// Create Service Usage client
	serviceUsageClient, err := serviceusage.NewService(ctx, option.WithCredentials(gcpClients.Credential))
	if err != nil {
		resp.Diagnostics.AddError(
			"[Enable API Services][Update]",
			fmt.Sprintf("Failed to create Service Usage client: %s", err),
		)
		return
	}

	// Enable each service concurrently
	var (
		mu              sync.Mutex
		wg              sync.WaitGroup
		billingWarnings []string
		enableErrors    []string
	)

	sem := cam.GCPServiceUsageSem
	for _, service := range services {
		wg.Add(1)
		go func(svc string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire slot
			defer func() { <-sem }() // release slot
			if err := r.enableService(ctx, serviceUsageClient, gcpClients.ProjectID, svc); err != nil {
				var be *billingError
				if errors.As(err, &be) {
					tflog.Warn(ctx, "Skipping service enable due to missing billing account", map[string]interface{}{
						"service":    svc,
						"project_id": gcpClients.ProjectID,
					})
					mu.Lock()
					billingWarnings = append(billingWarnings, svc)
					mu.Unlock()
					return
				}
				mu.Lock()
				enableErrors = append(enableErrors, fmt.Sprintf("Failed to enable service %s in project %s: %s", svc, gcpClients.ProjectID, err))
				mu.Unlock()
				return
			}
			tflog.Debug(ctx, "Successfully enabled service", map[string]interface{}{
				"service": svc,
			})
		}(service)
	}
	wg.Wait()

	if len(enableErrors) > 0 {
		resp.Diagnostics.AddError(
			"[Enable API Services][Update]",
			strings.Join(enableErrors, "; "),
		)
		return
	}

	if len(billingWarnings) > 0 {
		resp.Diagnostics.AddWarning(
			"[Enable API Services][Update] Billing not enabled",
			fmt.Sprintf("Project '%s' does not have a billing account linked. "+
				"The following services could not be enabled: %s. "+
				"Please link a billing account to the project and re-run, or enable these services manually in the GCP Console.",
				gcpClients.ProjectID, strings.Join(billingWarnings, ", ")),
		)
	}

	tflog.Info(ctx, "Successfully updated API services", map[string]interface{}{
		"project_id": gcpClients.ProjectID,
		"count":      len(services),
	})

	// Save state
	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
}

func (r *EnableAPIServices) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state enableAPIServicesResourceModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Info(ctx, "Deleting enable_api_services resource (services will not be disabled)", map[string]interface{}{
		"project_id": state.ProjectID.ValueString(),
	})

	// We intentionally do NOT disable the API services on delete
	// This matches the behavior of disable_on_destroy = false in the original template
	// The services remain enabled to prevent disruption to other resources
}

func (r *EnableAPIServices) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: project_id
	resource.ImportStatePassthroughID(ctx, path.Root("project_id"), req, resp)
}

// enableService enables a GCP API service and waits for the operation to complete.
// It retries with exponential backoff on 429 rate limit errors.
// Returns a sentinel error for billing-related failures so callers can handle them gracefully.
func (r *EnableAPIServices) enableService(ctx context.Context, client *serviceusage.Service, projectID, serviceName string) error {
	parent := fmt.Sprintf("projects/%s", projectID)
	servicePath := fmt.Sprintf("%s/services/%s", parent, serviceName)

	tflog.Debug(ctx, "Enabling service", map[string]interface{}{
		"service_path": servicePath,
	})

	// Pre-check: skip enable if the service is already enabled.
	// This avoids billing precondition errors when the customer has already
	// enabled the service via the GCP Console UI.
	alreadyEnabled, checkErr := r.isServiceEnabled(ctx, client, projectID, serviceName)
	if checkErr == nil && alreadyEnabled {
		tflog.Debug(ctx, "Service already enabled, skipping enable call", map[string]interface{}{
			"service": serviceName,
		})
		return nil
	}

	// Retry loop for 429 rate limit errors
	maxEnableRetries := 5
	baseBackoff := 5 * time.Second
	var operation *serviceusage.Operation
	var err error

	for attempt := 0; attempt <= maxEnableRetries; attempt++ {
		operation, err = client.Services.Enable(servicePath, &serviceusage.EnableServiceRequest{}).Context(ctx).Do()
		if err == nil {
			break
		}

		// Check if it's a billing precondition error — not retryable
		if isBillingError(err) {
			return &billingError{ProjectID: projectID, Err: err}
		}

		// Check if it's a rate limit error (429)
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rateLimitExceeded") || strings.Contains(err.Error(), "RATE_LIMIT_EXCEEDED") {
			if attempt < maxEnableRetries {
				backoff := baseBackoff * time.Duration(1<<attempt)
				if backoff > 2*time.Minute {
					backoff = 2 * time.Minute
				}
				tflog.Info(ctx, fmt.Sprintf("Rate limit hit for service %s, retrying in %v (attempt %d/%d)",
					serviceName, backoff, attempt+1, maxEnableRetries))
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
					continue
				}
			}
		}

		return fmt.Errorf("failed to initiate enable operation: %w", err)
	}

	// Wait for the operation to complete
	if operation.Done {
		tflog.Debug(ctx, "Service enable operation completed immediately", map[string]interface{}{
			"service": serviceName,
		})
		return nil
	}

	// Poll for operation completion
	maxRetries := 30
	retryInterval := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		time.Sleep(retryInterval)

		op, err := client.Operations.Get(operation.Name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get operation status: %w", err)
		}

		if op.Done {
			if op.Error != nil {
				return fmt.Errorf("operation failed: %s", op.Error.Message)
			}
			tflog.Debug(ctx, "Service enable operation completed", map[string]interface{}{
				"service": serviceName,
				"retries": i + 1,
			})
			return nil
		}

		tflog.Debug(ctx, "Waiting for service enable operation", map[string]interface{}{
			"service": serviceName,
			"attempt": i + 1,
		})
	}

	return fmt.Errorf("operation did not complete within timeout period")
}

// isServiceEnabled checks if a GCP API service is enabled
func (r *EnableAPIServices) isServiceEnabled(ctx context.Context, client *serviceusage.Service, projectID, serviceName string) (bool, error) {
	parent := fmt.Sprintf("projects/%s", projectID)
	servicePath := fmt.Sprintf("%s/services/%s", parent, serviceName)

	service, err := client.Services.Get(servicePath).Context(ctx).Do()
	if err != nil {
		return false, fmt.Errorf("failed to get service status: %w", err)
	}

	return service.State == "ENABLED", nil
}

// billingError represents a billing precondition failure when enabling GCP API services.
type billingError struct {
	ProjectID string
	Err       error
}

func (e *billingError) Error() string {
	return fmt.Sprintf("billing account for project '%s' is not found: %v", e.ProjectID, e.Err)
}

func (e *billingError) Unwrap() error {
	return e.Err
}

// isBillingError checks if the error is a GCP billing precondition failure.
func isBillingError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "UREQ_PROJECT_BILLING_NOT_FOUND") ||
		strings.Contains(errMsg, "billing-enabled") ||
		strings.Contains(errMsg, "Billing must be enabled") ||
		strings.Contains(errMsg, "Billing account for project")
}
