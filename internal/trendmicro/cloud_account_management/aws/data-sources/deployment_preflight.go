package data_sources

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/aws/data-sources/config"
)

var _ datasource.DataSource = &CAMDeploymentPreflightDataSource{}

type stsAPI interface {
	GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

type iamAPI interface {
	SimulatePrincipalPolicy(context.Context, *iam.SimulatePrincipalPolicyInput, ...func(*iam.Options)) (*iam.SimulatePrincipalPolicyOutput, error)
}

type organizationsAPI interface {
	DescribeOrganization(context.Context, *organizations.DescribeOrganizationInput, ...func(*organizations.Options)) (*organizations.DescribeOrganizationOutput, error)
}

type awsClientFactory func(context.Context, string) (stsAPI, iamAPI, organizationsAPI, error)

// CAMDeploymentPreflightDataSource checks the permissions needed by the
// Terraform caller before any deployment resource is created.
type CAMDeploymentPreflightDataSource struct {
	clientFactory awsClientFactory
}

type deploymentPermissionModel struct {
	Actions            tftypes.List   `tfsdk:"actions"`
	Resources          tftypes.List   `tfsdk:"resources"`
	ExecutionPrincipal tftypes.String `tfsdk:"execution_principal"`
}

type CAMDeploymentPreflightDataSourceModel struct {
	DeploymentType        tftypes.String `tfsdk:"deployment_type"`
	AWSRegion             tftypes.String `tfsdk:"aws_region"`
	CloudAccountID        tftypes.String `tfsdk:"cloud_account_id"`
	ExpectedCallerARN     tftypes.String `tfsdk:"expected_caller_arn"`
	DeploymentPermissions tftypes.List   `tfsdk:"deployment_permissions"`

	ID                  tftypes.String `tfsdk:"id"`
	Status              tftypes.String `tfsdk:"status"`
	CallerARN           tftypes.String `tfsdk:"caller_arn"`
	AccountID           tftypes.String `tfsdk:"account_id"`
	Partition           tftypes.String `tfsdk:"partition"`
	CheckedPermissions  tftypes.List   `tfsdk:"checked_permissions"`
	FailedPermissions   tftypes.List   `tfsdk:"failed_permissions"`
	ManagementAccountID tftypes.String `tfsdk:"management_account_id"`
	IsManagementAccount tftypes.Bool   `tfsdk:"is_management_account"`
	Warnings            tftypes.List   `tfsdk:"warnings"`
}

type permissionRequirement struct {
	actions   []string
	resources []string
}

type preflightResult struct {
	callerARN           string
	accountID           string
	partition           string
	checked             []string
	failed              []string
	managementAccountID string
	isManagementAccount bool
	warnings            []string
}

func NewCAMDeploymentPreflightDataSource() datasource.DataSource {
	return &CAMDeploymentPreflightDataSource{clientFactory: defaultAWSClientFactory}
}

func (d *CAMDeploymentPreflightDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.DATA_SOURCE_TYPE_DEPLOYMENT_PREFLIGHT
}

func (d *CAMDeploymentPreflightDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Checks AWS deployer permissions during terraform plan without creating AWS resources. Organization deployments also verify that the selected account is the AWS Organizations management account.",
		Attributes: map[string]schema.Attribute{
			"deployment_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Deployment context. Supports `single` and `organization` deployments.",
				Validators: []validator.String{
					stringvalidator.OneOf("single", "organization"),
				},
			},
			"aws_region": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "AWS region used to load the SDK credential chain and validate the partition.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"cloud_account_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "AWS account ID selected for deployment. Verified against the caller used by this data source and, for `organization` deployments, the AWS Organizations management account.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^\d{12}$`), "must be a 12-digit AWS account ID"),
				},
			},
			"expected_caller_arn": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Caller ARN returned by the `hashicorp/aws` provider. The preflight AWS SDK credentials must resolve to the same IAM principal.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"deployment_permissions": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Permission groups to simulate for the Terraform caller.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"actions": schema.ListAttribute{
							Required:            true,
							ElementType:         tftypes.StringType,
							MarkdownDescription: "AWS API actions to simulate.",
						},
						"resources": schema.ListAttribute{
							Required:            true,
							ElementType:         tftypes.StringType,
							MarkdownDescription: "Resource ARNs or `*` to simulate.",
						},
						"execution_principal": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Optional execution principal label. The supported value is `terraform_caller`.",
						},
					},
				},
			},
			"id": schema.StringAttribute{
				Computed: true,
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`passed` when every simulated permission is allowed.",
			},
			"caller_arn": schema.StringAttribute{
				Computed: true,
			},
			"account_id": schema.StringAttribute{
				Computed: true,
			},
			"partition": schema.StringAttribute{
				Computed: true,
			},
			"checked_permissions": schema.ListAttribute{
				Computed:    true,
				ElementType: tftypes.StringType,
			},
			"failed_permissions": schema.ListAttribute{
				Computed:    true,
				ElementType: tftypes.StringType,
			},
			"management_account_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "AWS Organizations management account ID returned by `organizations:DescribeOrganization` for an organization deployment.",
			},
			"is_management_account": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether `cloud_account_id` is the AWS Organizations management account.",
			},
			"warnings": schema.ListAttribute{
				Computed:    true,
				ElementType: tftypes.StringType,
			},
		},
	}
}

func (d *CAMDeploymentPreflightDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CAMDeploymentPreflightDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deploymentType := data.DeploymentType.ValueString()
	cloudAccountID := strings.TrimSpace(data.CloudAccountID.ValueString())
	if cloudAccountID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("cloud_account_id"),
			"Missing AWS account ID",
			"cloud_account_id is required so the provider can verify the AWS deployment target.",
		)
		return
	}
	expectedCallerARN := strings.TrimSpace(data.ExpectedCallerARN.ValueString())
	if expectedCallerARN == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("expected_caller_arn"),
			"Missing expected AWS caller ARN",
			"expected_caller_arn must be the caller ARN returned by the hashicorp/aws provider so the preflight credentials can be verified.",
		)
		return
	}

	var permissionModels []deploymentPermissionModel
	resp.Diagnostics.Append(data.DeploymentPermissions.ElementsAs(ctx, &permissionModels, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requirements, err := normalizePermissionModels(ctx, permissionModels)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("deployment_permissions"),
			"Invalid deployment permissions",
			err.Error(),
		)
		return
	}

	stsClient, iamClient, organizationsClient, err := d.clientFactory(ctx, data.AWSRegion.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure AWS SDK", err.Error())
		return
	}

	result, err := executePreflight(ctx, stsClient, iamClient, data.AWSRegion.ValueString(), requirements)
	if err != nil {
		resp.Diagnostics.AddError(
			"AWS deployment permission preflight failed",
			err.Error(),
		)
		return
	}

	if len(result.failed) > 0 {
		data.ID = tftypes.StringValue(fmt.Sprintf("%s/%s", data.DeploymentType.ValueString(), data.AWSRegion.ValueString()))
		data.Status = tftypes.StringValue("failed")
		data.CallerARN = tftypes.StringValue(result.callerARN)
		data.AccountID = tftypes.StringValue(result.accountID)
		data.Partition = tftypes.StringValue(result.partition)
		data.CheckedPermissions = stringListValue(result.checked)
		data.FailedPermissions = stringListValue(result.failed)
		data.Warnings = stringListValue(result.warnings)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		resp.Diagnostics.AddError(
			"AWS deployment permissions are insufficient",
			formatInsufficientPermissionsError(result.callerARN, deploymentType, result.failed),
		)
		return
	}

	if err := validateCallerAccount(result.accountID, cloudAccountID); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("cloud_account_id"),
			"AWS caller account does not match the selected account",
			err.Error(),
		)
		return
	}
	if err := validateCallerIdentity(result.callerARN, expectedCallerARN); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("expected_caller_arn"),
			"AWS credentials do not match the Terraform AWS provider",
			err.Error(),
		)
		return
	}

	if deploymentType == "organization" {
		managementAccountID, isManagementAccount, checkErr := checkOrganizationManagementAccount(ctx, organizationsClient, cloudAccountID)
		if checkErr != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("cloud_account_id"),
				"AWS Organizations management account check failed",
				checkErr.Error(),
			)
			return
		}
		result.managementAccountID = managementAccountID
		result.isManagementAccount = isManagementAccount
		if !isManagementAccount {
			resp.Diagnostics.AddAttributeError(
				path.Root("cloud_account_id"),
				"Selected account is not the AWS Organizations management account",
				fmt.Sprintf("cloud_account_id %q is a member account. AWS Organizations returned MasterAccountId %q; organization deployments must select the management account.", cloudAccountID, managementAccountID),
			)
			return
		}
	}

	data.ID = tftypes.StringValue(fmt.Sprintf("%s/%s", data.DeploymentType.ValueString(), data.AWSRegion.ValueString()))
	data.Status = tftypes.StringValue("passed")
	data.CallerARN = tftypes.StringValue(result.callerARN)
	data.AccountID = tftypes.StringValue(result.accountID)
	data.Partition = tftypes.StringValue(result.partition)
	data.CheckedPermissions = stringListValue(result.checked)
	data.FailedPermissions = stringListValue(result.failed)
	if result.managementAccountID != "" {
		data.ManagementAccountID = tftypes.StringValue(result.managementAccountID)
		data.IsManagementAccount = tftypes.BoolValue(result.isManagementAccount)
	} else {
		data.ManagementAccountID = tftypes.StringNull()
		data.IsManagementAccount = tftypes.BoolValue(false)
	}
	data.Warnings = stringListValue(result.warnings)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func defaultAWSClientFactory(ctx context.Context, region string) (stsAPI, iamAPI, organizationsAPI, error) {
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load AWS default credential chain: %w", err)
	}

	return sts.NewFromConfig(awsConfig), iam.NewFromConfig(awsConfig), organizations.NewFromConfig(awsConfig), nil
}

func checkOrganizationManagementAccount(ctx context.Context, client organizationsAPI, cloudAccountID string) (managementAccountID string, isManagementAccount bool, err error) {
	organization, err := client.DescribeOrganization(ctx, &organizations.DescribeOrganizationInput{})
	if err != nil {
		return "", false, fmt.Errorf("organizations:DescribeOrganization failed: %w", err)
	}
	if organization == nil || organization.Organization == nil || organization.Organization.MasterAccountId == nil || aws.ToString(organization.Organization.MasterAccountId) == "" {
		return "", false, fmt.Errorf("organizations:DescribeOrganization returned no management account ID")
	}

	managementAccountID = aws.ToString(organization.Organization.MasterAccountId)
	return managementAccountID, cloudAccountID == managementAccountID, nil
}

func validateCallerAccount(callerAccountID, cloudAccountID string) error {
	if strings.TrimSpace(callerAccountID) != strings.TrimSpace(cloudAccountID) {
		return fmt.Errorf("STS caller account %q does not match cloud_account_id %q; deployment must run with credentials from the selected account", callerAccountID, cloudAccountID)
	}
	return nil
}

func validateCallerIdentity(callerARN, expectedCallerARN string) error {
	callerPrincipalARN, err := normalizePrincipalARN(callerARN)
	if err != nil {
		return err
	}
	expectedPrincipalARN, err := normalizePrincipalARN(expectedCallerARN)
	if err != nil {
		return fmt.Errorf("parse expected Terraform AWS caller ARN %q: %w", expectedCallerARN, err)
	}
	if callerPrincipalARN != expectedPrincipalARN {
		return fmt.Errorf("AWS SDK caller %q does not match Terraform AWS caller %q after IAM principal normalization (%q != %q)", callerARN, expectedCallerARN, callerPrincipalARN, expectedPrincipalARN)
	}
	return nil
}

func normalizePermissionModels(ctx context.Context, models []deploymentPermissionModel) ([]permissionRequirement, error) {
	requirements := make([]permissionRequirement, 0, len(models))
	for index, model := range models {
		var actions, resources []string
		if diags := model.Actions.ElementsAs(ctx, &actions, false); diags.HasError() {
			return nil, fmt.Errorf("permission group %d has invalid actions", index)
		}
		if diags := model.Resources.ElementsAs(ctx, &resources, false); diags.HasError() {
			return nil, fmt.Errorf("permission group %d has invalid resources", index)
		}
		if len(actions) == 0 || len(resources) == 0 {
			return nil, fmt.Errorf("permission group %d must contain at least one action and resource", index)
		}
		for i := range actions {
			actions[i] = strings.TrimSpace(actions[i])
			if actions[i] == "" {
				return nil, fmt.Errorf("permission group %d contains an empty action", index)
			}
		}
		for i := range resources {
			resources[i] = strings.TrimSpace(resources[i])
			if resources[i] == "" {
				return nil, fmt.Errorf("permission group %d contains an empty resource", index)
			}
		}
		if model.ExecutionPrincipal.IsUnknown() {
			return nil, fmt.Errorf("permission group %d has an unknown execution_principal", index)
		}
		if !model.ExecutionPrincipal.IsNull() && model.ExecutionPrincipal.ValueString() != "terraform_caller" {
			return nil, fmt.Errorf("permission group %d has unsupported execution_principal %q", index, model.ExecutionPrincipal.ValueString())
		}
		requirements = append(requirements, permissionRequirement{actions: actions, resources: resources})
	}
	if len(requirements) == 0 {
		return nil, fmt.Errorf("deployment_permissions must contain at least one permission group")
	}
	return requirements, nil
}

func executePreflight(ctx context.Context, stsClient stsAPI, iamClient iamAPI, region string, requirements []permissionRequirement) (*preflightResult, error) {
	identity, stsErr := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if stsErr != nil {
		return nil, fmt.Errorf("sts:GetCallerIdentity failed: %w", stsErr)
	}
	if identity == nil || identity.Arn == nil || identity.Account == nil {
		return nil, fmt.Errorf("sts:GetCallerIdentity returned an incomplete caller identity")
	}

	callerARN := aws.ToString(identity.Arn)
	caller, parseErr := arn.Parse(callerARN)
	if parseErr != nil {
		return nil, fmt.Errorf("parse caller ARN %q: %w", callerARN, parseErr)
	}
	if err := validateRegionPartition(region, caller.Partition); err != nil {
		return nil, err
	}
	principalARN, err := normalizePrincipalARN(callerARN)
	if err != nil {
		return nil, err
	}

	result := &preflightResult{
		callerARN: callerARN,
		accountID: aws.ToString(identity.Account),
		partition: caller.Partition,
		checked:   make([]string, 0),
		failed:    make([]string, 0),
		warnings:  make([]string, 0),
	}

	for _, requirement := range requirements {
		paginator := iam.NewSimulatePrincipalPolicyPaginator(iamClient, &iam.SimulatePrincipalPolicyInput{
			PolicySourceArn: aws.String(principalARN),
			ActionNames:     requirement.actions,
			ResourceArns:    requirement.resources,
		})
		evaluations := make(map[string]*iamtypes.EvaluationResult)
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, formatSimulationError(principalARN, err)
			}
			if output == nil {
				return nil, fmt.Errorf("iam:SimulatePrincipalPolicy returned an empty response for principal %s", principalARN)
			}
			for i := range output.EvaluationResults {
				evaluation := &output.EvaluationResults[i]
				evaluations[aws.ToString(evaluation.EvalActionName)] = evaluation
			}
		}
		for _, action := range requirement.actions {
			evaluation, ok := evaluations[action]
			for _, resource := range requirement.resources {
				result.checked = append(result.checked, permissionKey(action, resource))
				if !ok {
					result.failed = append(result.failed, fmt.Sprintf("%s (no simulation result)", permissionKey(action, resource)))
					continue
				}

				decision := evaluation.EvalDecision
				missingContextValues := evaluation.MissingContextValues
				if len(evaluation.ResourceSpecificResults) > 0 {
					resourceResult, resourceOK := findResourceSpecificResult(evaluation.ResourceSpecificResults, resource)
					if !resourceOK {
						result.failed = append(result.failed, fmt.Sprintf("%s (no resource-specific simulation result)", permissionKey(action, resource)))
						continue
					}
					decision = resourceResult.EvalResourceDecision
					missingContextValues = resourceResult.MissingContextValues
				}
				if len(missingContextValues) > 0 {
					result.warnings = append(result.warnings, fmt.Sprintf("%s has missing simulation context: %s", permissionKey(action, resource), strings.Join(missingContextValues, ",")))
				}

				if decision != iamtypes.PolicyEvaluationDecisionTypeAllowed {
					result.failed = append(result.failed, formatEvaluationFailure(
						action,
						resource,
						decision,
						missingContextValues,
						evaluation.OrganizationsDecisionDetail,
					))
				}
			}
		}
	}

	return result, nil
}

func normalizePrincipalARN(callerARN string) (string, error) {
	parsed, err := arn.Parse(callerARN)
	if err != nil {
		return "", fmt.Errorf("parse caller ARN %q: %w", callerARN, err)
	}

	switch {
	case parsed.Service == "iam" && (strings.HasPrefix(parsed.Resource, "role/") || strings.HasPrefix(parsed.Resource, "user/")):
		return callerARN, nil
	case parsed.Service == "sts" && strings.HasPrefix(parsed.Resource, "assumed-role/"):
		roleAndSession := strings.TrimPrefix(parsed.Resource, "assumed-role/")
		separator := strings.LastIndex(roleAndSession, "/")
		if separator <= 0 {
			break
		}
		roleName := roleAndSession[:separator]
		return fmt.Sprintf("arn:%s:iam::%s:role/%s", parsed.Partition, parsed.AccountID, roleName), nil
	}

	return "", fmt.Errorf("caller ARN %q cannot be used as an IAM simulation principal; use an IAM user or role credential", callerARN)
}

func validateRegionPartition(region, partition string) error {
	expected := "aws"
	switch {
	case strings.HasPrefix(region, "cn-"):
		expected = "aws-cn"
	case strings.HasPrefix(region, "us-gov-"):
		expected = "aws-us-gov"
	case strings.HasPrefix(region, "us-isob-"):
		expected = "aws-isob"
	case strings.HasPrefix(region, "us-iso-"):
		expected = "aws-iso"
	case strings.HasPrefix(region, "eu-isoe-"):
		expected = "aws-iso-e"
	}
	if partition != expected {
		return fmt.Errorf("AWS region %q belongs to partition %q, but caller ARN belongs to %q", region, expected, partition)
	}
	return nil
}

func permissionKey(action, resource string) string {
	return fmt.Sprintf("%s on %s", action, resource)
}

func formatSimulationError(principalARN string, err error) error {
	if isSimulationPermissionDenied(err) {
		return fmt.Errorf("the Terraform caller lacks iam:SimulatePrincipalPolicy, which is required to run the deployment preflight for principal %s; grant iam:SimulatePrincipalPolicy to the caller and run terraform plan again: %w", principalARN, err)
	}
	return fmt.Errorf("iam:SimulatePrincipalPolicy failed for principal %s: %w", principalARN, err)
}

func isSimulationPermissionDenied(err error) bool {
	var apiErr interface{ ErrorCode() string }
	if !errors.As(err, &apiErr) {
		return false
	}

	switch apiErr.ErrorCode() {
	case "AccessDenied", "AccessDeniedException", "UnauthorizedOperation":
		return true
	default:
		return false
	}
}

func findResourceSpecificResult(results []iamtypes.ResourceSpecificResult, resource string) (iamtypes.ResourceSpecificResult, bool) {
	for _, result := range results {
		if resourcePatternMatches(resource, aws.ToString(result.EvalResourceName)) {
			return result, true
		}
	}
	return iamtypes.ResourceSpecificResult{}, false
}

func resourcePatternMatches(pattern, resource string) bool {
	if pattern == resource {
		return true
	}
	if !strings.ContainsAny(pattern, "*?") {
		return false
	}

	expression := regexp.QuoteMeta(pattern)
	expression = strings.ReplaceAll(expression, `\*`, `.*`)
	expression = strings.ReplaceAll(expression, `\?`, `.`)
	matched, _ := regexp.MatchString("^"+expression+"$", resource)
	return matched
}

func formatEvaluationFailure(action, resource string, decision iamtypes.PolicyEvaluationDecisionType, missingContextValues []string, organizationsDecision *iamtypes.OrganizationsDecisionDetail) string {
	details := []string{string(decision)}
	if organizationsDecision != nil && !organizationsDecision.AllowedByOrganizations {
		details = append(details, "organizations_scp=denied")
	}
	if len(missingContextValues) > 0 {
		details = append(details, "missing_context="+strings.Join(missingContextValues, ","))
	}
	return fmt.Sprintf("%s (%s)", permissionKey(action, resource), strings.Join(details, ", "))
}

func formatInsufficientPermissionsError(callerARN, deploymentType string, failed []string) string {
	var builder strings.Builder

	builder.WriteString("The Terraform caller cannot deploy this CAM package.\n")
	fmt.Fprintf(&builder, "Caller: %s\n", callerARN)
	fmt.Fprintf(&builder, "Deployment type: %s\n", deploymentType)
	fmt.Fprintf(&builder, "Failed permission checks: %d\n", len(failed))
	builder.WriteString("\nMissing or denied permissions by AWS service:\n")

	type permissionGroup struct {
		service     string
		permissions []string
	}
	groups := make([]permissionGroup, 0)
	groupIndexes := make(map[string]int)
	for _, failure := range failed {
		permission, _, _ := strings.Cut(failure, " (")
		action, resource, ok := strings.Cut(permission, " on ")
		if !ok {
			groups = append(groups, permissionGroup{service: "other", permissions: []string{failure}})
			continue
		}
		service, actionName, ok := strings.Cut(action, ":")
		if !ok {
			service = "other"
			actionName = action
		}
		index, exists := groupIndexes[service]
		if !exists {
			index = len(groups)
			groupIndexes[service] = index
			groups = append(groups, permissionGroup{service: service})
		}
		groups[index].permissions = append(groups[index].permissions, fmt.Sprintf("%s on %s", actionName, resource))
	}

	for _, group := range groups {
		fmt.Fprintf(&builder, "  %s:\n", group.service)
		for _, permission := range group.permissions {
			fmt.Fprintf(&builder, "    - %s\n", permission)
		}
	}

	builder.WriteString("\nWhat to do:\n")
	builder.WriteString("  1. Use AWS credentials for an IAM role or user intended for CAM Terraform deployment; read-only credentials cannot deploy this package.\n")
	builder.WriteString("  2. Grant the required actions and resource scopes listed above. The complete package requirement is in terraform.tfvars under deployment_permissions.\n")
	builder.WriteString("  3. If these permissions are already attached, check permission boundaries, AWS Organizations SCPs, and session policies for explicit denies.\n")
	builder.WriteString("  4. If a missing_context value is shown below, review policy conditions that use that context key; the simulation could not evaluate those conditions with the available context.\n")
	if deploymentType == "organization" {
		builder.WriteString("  5. For organization deployments, run Terraform with credentials from the AWS Organizations management account.\n")
		builder.WriteString("  6. Run terraform plan again after updating the credentials or policies.\n")
	} else {
		builder.WriteString("  5. Run terraform plan again after updating the credentials or policies.\n")
	}

	details := make([]string, 0)
	seenDetails := make(map[string]struct{})
	for _, failure := range failed {
		_, detail, ok := strings.Cut(failure, " (")
		if !ok {
			continue
		}
		detail = strings.TrimSuffix(detail, ")")
		if _, exists := seenDetails[detail]; exists {
			continue
		}
		seenDetails[detail] = struct{}{}
		details = append(details, detail)
	}
	if len(details) > 0 {
		builder.WriteString("\nAWS simulation details:\n")
		for _, detail := range details {
			fmt.Fprintf(&builder, "  - %s\n", detail)
		}
	}

	return strings.TrimSuffix(builder.String(), "\n")
}

func stringListValue(values []string) tftypes.List {
	value, _ := tftypes.ListValueFrom(context.Background(), tftypes.StringType, values)
	return value
}
