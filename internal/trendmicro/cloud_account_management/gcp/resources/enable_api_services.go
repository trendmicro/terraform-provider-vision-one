package resources

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	ProjectID         types.String `tfsdk:"project_id"`
	Services          types.List   `tfsdk:"services"`
	EnabledByResource types.List   `tfsdk:"enabled_by_resource"`
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
			"Only services that were disabled before creation are enabled by this resource; those services will be disabled on destroy.",
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
				Default:  listdefault.StaticValue(cam.SortedListValue(config.GCP_REQUIRED_ENABLE_API_AND_SERVICE)),
			},
			"enabled_by_resource": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of GCP API service names that were disabled before this resource was created and were enabled by this resource. These services will be disabled when this resource is destroyed.",
				Computed:            true,
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
	sort.Strings(services)

	tflog.Info(ctx, "Enabling GCP API services", map[string]interface{}{
		"project_id": gcpClients.ProjectID,
		"services":   services,
	})

	serviceUsageClient, err := serviceusage.NewService(ctx, option.WithCredentials(gcpClients.Credential))
	if err != nil {
		resp.Diagnostics.AddError(
			"[Enable API Services][Create]",
			fmt.Sprintf("Failed to create Service Usage client: %s", err),
		)
		return
	}

	servicesToEnable := r.filterDisabledServices(ctx, serviceUsageClient, gcpClients.ProjectID, services)
	enabledByUs, billingWarnings, enableErrors := r.runEnableServices(ctx, serviceUsageClient, gcpClients.ProjectID, servicesToEnable)

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

	tflog.Info(ctx, "Successfully enabled API services", map[string]interface{}{
		"project_id":          gcpClients.ProjectID,
		"enabled_by_resource": enabledByUs,
	})

	sort.Strings(enabledByUs)
	plan.Services = cam.ConvertStringSliceToListValue(services)
	plan.EnabledByResource = cam.ConvertStringSliceToListValue(enabledByUs)

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
	sort.Strings(services)

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
	var state enableAPIServicesResourceModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	projectID := plan.ProjectID.ValueString()
	gcpClients, diags := api.GetGCPClients(ctx, projectID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	var newServices, oldServices, prevEnabledByResource []string
	if diagsList := plan.Services.ElementsAs(ctx, &newServices, false); diagsList.HasError() {
		resp.Diagnostics.Append(diagsList...)
		return
	}
	if diagsList := state.Services.ElementsAs(ctx, &oldServices, false); diagsList.HasError() {
		resp.Diagnostics.Append(diagsList...)
		return
	}
	if diagsList := state.EnabledByResource.ElementsAs(ctx, &prevEnabledByResource, false); diagsList.HasError() {
		resp.Diagnostics.Append(diagsList...)
		return
	}
	sort.Strings(newServices)

	addedServices, removedAndOwnedServices, newSet := computeServiceDiff(newServices, oldServices, prevEnabledByResource)

	serviceUsageClient, err := serviceusage.NewService(ctx, option.WithCredentials(gcpClients.Credential))
	if err != nil {
		resp.Diagnostics.AddError(
			"[Enable API Services][Update]",
			fmt.Sprintf("Failed to create Service Usage client: %s", err),
		)
		return
	}

	servicesToEnable := r.filterDisabledServices(ctx, serviceUsageClient, gcpClients.ProjectID, addedServices)

	newlyEnabled, billingWarnings, enableErrors := r.runEnableServices(ctx, serviceUsageClient, gcpClients.ProjectID, servicesToEnable)
	disableErrors := r.runDisableServices(ctx, serviceUsageClient, gcpClients.ProjectID, removedAndOwnedServices)

	if len(enableErrors) > 0 || len(disableErrors) > 0 {
		resp.Diagnostics.AddError(
			"[Enable API Services][Update]",
			strings.Join(append(enableErrors, disableErrors...), "; "),
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

	updatedEnabledByResource := make([]string, 0)
	for _, s := range prevEnabledByResource {
		if newSet[s] {
			updatedEnabledByResource = append(updatedEnabledByResource, s)
		}
	}
	updatedEnabledByResource = append(updatedEnabledByResource, newlyEnabled...)
	sort.Strings(updatedEnabledByResource)
	plan.Services = cam.ConvertStringSliceToListValue(newServices)
	plan.EnabledByResource = cam.ConvertStringSliceToListValue(updatedEnabledByResource)

	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
}

// computeServiceDiff returns added services, removed-and-owned services, and the new services set.
func computeServiceDiff(newServices, oldServices, prevEnabledByResource []string) (added, removedOwned []string, newSet map[string]bool) {
	sort.Strings(oldServices)
	sort.Strings(prevEnabledByResource)

	oldSet := make(map[string]bool, len(oldServices))
	for _, s := range oldServices {
		oldSet[s] = true
	}
	newSet = make(map[string]bool, len(newServices))
	for _, s := range newServices {
		newSet[s] = true
	}
	prevEnabledSet := make(map[string]bool, len(prevEnabledByResource))
	for _, s := range prevEnabledByResource {
		prevEnabledSet[s] = true
	}

	for _, s := range newServices {
		if !oldSet[s] {
			added = append(added, s)
		}
	}
	for _, s := range oldServices {
		if !newSet[s] && prevEnabledSet[s] {
			removedOwned = append(removedOwned, s)
		}
	}
	return added, removedOwned, newSet
}

// filterDisabledServices returns only services from the input list that are currently disabled.
func (r *EnableAPIServices) filterDisabledServices(ctx context.Context, client *serviceusage.Service, projectID string, services []string) []string {
	if len(services) == 0 {
		return nil
	}
	type checkResult struct {
		service string
		enabled bool
		err     error
	}
	results := make([]checkResult, len(services))
	var wg sync.WaitGroup
	sem := cam.GCPServiceUsageSem
	for i, svc := range services {
		wg.Add(1)
		go func(idx int, s string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			enabled, err := r.isServiceEnabled(ctx, client, projectID, s)
			results[idx] = checkResult{service: s, enabled: enabled, err: err}
		}(i, svc)
	}
	wg.Wait()

	var disabled []string
	for _, res := range results {
		if res.err != nil || !res.enabled {
			disabled = append(disabled, res.service)
		}
	}
	return disabled
}

// runEnableServices concurrently enables services and returns (newlyEnabled, billingWarnings, errors).
func (r *EnableAPIServices) runEnableServices(ctx context.Context, client *serviceusage.Service, projectID string, services []string) (enabled, billingWarnings, errs []string) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := cam.GCPServiceUsageSem
	for _, service := range services {
		wg.Add(1)
		go func(svc string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := r.enableService(ctx, client, projectID, svc); err != nil {
				var be *billingError
				if errors.As(err, &be) {
					tflog.Warn(ctx, "Skipping service enable due to missing billing account", map[string]interface{}{
						"service": svc, "project_id": projectID,
					})
					mu.Lock()
					billingWarnings = append(billingWarnings, svc)
					mu.Unlock()
					return
				}
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Failed to enable service %s: %s", svc, err))
				mu.Unlock()
				return
			}
			mu.Lock()
			enabled = append(enabled, svc)
			mu.Unlock()
		}(service)
	}
	wg.Wait()
	return enabled, billingWarnings, errs
}

// runDisableServices concurrently disables services and returns any errors.
func (r *EnableAPIServices) runDisableServices(ctx context.Context, client *serviceusage.Service, projectID string, services []string) []string {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errs []string
	sem := cam.GCPServiceUsageSem
	for _, service := range services {
		wg.Add(1)
		go func(svc string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := r.disableService(ctx, client, projectID, svc); err != nil {
				tflog.Warn(ctx, "Failed to disable removed service", map[string]interface{}{
					"service": svc, "error": err.Error(),
				})
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Failed to disable service %s: %s", svc, err))
				mu.Unlock()
			}
		}(service)
	}
	wg.Wait()
	return errs
}

func (r *EnableAPIServices) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state enableAPIServicesResourceModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Only disable services that this resource originally enabled (not pre-existing ones)
	var enabledByResource []string
	if diagsList := state.EnabledByResource.ElementsAs(ctx, &enabledByResource, false); diagsList.HasError() {
		resp.Diagnostics.Append(diagsList...)
		return
	}

	if len(enabledByResource) == 0 {
		tflog.Info(ctx, "No services were enabled by this resource, nothing to disable", map[string]interface{}{
			"project_id": state.ProjectID.ValueString(),
		})
		return
	}

	tflog.Info(ctx, "Disabling GCP API services that were enabled by this resource", map[string]interface{}{
		"project_id": state.ProjectID.ValueString(),
		"services":   enabledByResource,
	})

	projectID := state.ProjectID.ValueString()
	gcpClients, diags := api.GetGCPClients(ctx, projectID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	serviceUsageClient, err := serviceusage.NewService(ctx, option.WithCredentials(gcpClients.Credential))
	if err != nil {
		resp.Diagnostics.AddError(
			"[Enable API Services][Delete]",
			fmt.Sprintf("Failed to create Service Usage client: %s", err),
		)
		return
	}

	disableErrors := r.runDisableServices(ctx, serviceUsageClient, gcpClients.ProjectID, enabledByResource)
	if len(disableErrors) > 0 {
		resp.Diagnostics.AddError(
			"[Enable API Services][Delete]",
			strings.Join(disableErrors, "; "),
		)
	}
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
	maxRetries := 15
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

// disableService disables a GCP API service and waits for the operation to complete.
// It retries with exponential backoff on 429 rate limit errors.
func (r *EnableAPIServices) disableService(ctx context.Context, client *serviceusage.Service, projectID, serviceName string) error {
	parent := fmt.Sprintf("projects/%s", projectID)
	servicePath := fmt.Sprintf("%s/services/%s", parent, serviceName)

	maxDisableRetries := 5
	baseBackoff := 5 * time.Second
	var operation *serviceusage.Operation
	var err error

	for attempt := 0; attempt <= maxDisableRetries; attempt++ {
		operation, err = client.Services.Disable(servicePath, &serviceusage.DisableServiceRequest{
			DisableDependentServices: true,
		}).Context(ctx).Do()
		if err == nil {
			break
		}

		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rateLimitExceeded") || strings.Contains(err.Error(), "RATE_LIMIT_EXCEEDED") {
			if attempt < maxDisableRetries {
				backoff := baseBackoff * time.Duration(1<<attempt)
				if backoff > 2*time.Minute {
					backoff = 2 * time.Minute
				}
				tflog.Info(ctx, fmt.Sprintf("Rate limit hit for service %s, retrying in %v (attempt %d/%d)",
					serviceName, backoff, attempt+1, maxDisableRetries))
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
					continue
				}
			}
		}

		return fmt.Errorf("failed to initiate disable operation: %w", err)
	}

	if operation.Done {
		return nil
	}

	maxRetries := 15
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
			return nil
		}
	}

	return fmt.Errorf("disable operation did not complete within timeout period")
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
