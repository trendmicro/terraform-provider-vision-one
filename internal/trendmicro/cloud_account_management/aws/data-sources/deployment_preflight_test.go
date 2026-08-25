package data_sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationstypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	frameworktypes "github.com/hashicorp/terraform-plugin-framework/types"
	terraformtypes "github.com/hashicorp/terraform-plugin-go/tftypes"
	"terraform-provider-vision-one/internal/trendmicro"
	awsapi "terraform-provider-vision-one/internal/trendmicro/cloud_account_management/aws/data-sources/api"
)

type mockSTSClient struct {
	identity *sts.GetCallerIdentityOutput
	err      error
}

func (m mockSTSClient) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return m.identity, m.err
}

type mockIAMClient struct {
	output *iam.SimulatePrincipalPolicyOutput
	next   *iam.SimulatePrincipalPolicyOutput
	err    error
}

func (m mockIAMClient) SimulatePrincipalPolicy(_ context.Context, input *iam.SimulatePrincipalPolicyInput, _ ...func(*iam.Options)) (*iam.SimulatePrincipalPolicyOutput, error) {
	if input.Marker != nil && m.next != nil {
		return m.next, m.err
	}
	return m.output, m.err
}

type mockOrganizationsClient struct {
	output *organizations.DescribeOrganizationOutput
	err    error
}

func (m mockOrganizationsClient) DescribeOrganization(context.Context, *organizations.DescribeOrganizationInput, ...func(*organizations.Options)) (*organizations.DescribeOrganizationOutput, error) {
	return m.output, m.err
}

type mockAWSAPIError struct {
	code string
}

func (e mockAWSAPIError) Error() string {
	return e.code
}

func (e mockAWSAPIError) ErrorCode() string {
	return e.code
}

func TestNormalizePrincipalARN(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "assumed role",
			input: "arn:aws:sts::123456789012:assumed-role/path/TerraformRole/session",
			want:  "arn:aws:iam::123456789012:role/path/TerraformRole",
		},
		{
			name:  "iam user",
			input: "arn:aws:iam::123456789012:user/terraform",
			want:  "arn:aws:iam::123456789012:user/terraform",
		},
		{
			name:    "federated user",
			input:   "arn:aws:sts::123456789012:federated-user/terraform",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePrincipalARN(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizePrincipalARN() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("normalizePrincipalARN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCAMDeploymentPreflightDataSourceMetadataAndSchema(t *testing.T) {
	dataSource := NewCAMDeploymentPreflightDataSource().(*CAMDeploymentPreflightDataSource)
	metadataResponse := datasource.MetadataResponse{}
	dataSource.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "visionone"}, &metadataResponse)
	if metadataResponse.TypeName != "visionone_cam_aws_deployment_preflight" {
		t.Fatalf("data source type name = %q", metadataResponse.TypeName)
	}

	schemaResponse := datasource.SchemaResponse{}
	dataSource.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResponse)
	if len(schemaResponse.Schema.GetAttributes()) == 0 {
		t.Fatal("data source schema has no attributes")
	}
}

func TestCAMCloudAccountsDataSourceMetadataSchemaAndConfigure(t *testing.T) {
	if NewCAMCloudAccountsDataSource() == nil {
		t.Fatal("NewCAMCloudAccountsDataSource() returned nil")
	}
	dataSource := &CAMCloudAccountsDataSource{}
	metadataResponse := datasource.MetadataResponse{}
	dataSource.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "visionone"}, &metadataResponse)
	if metadataResponse.TypeName != "visionone_cam_connect_aws_accounts" {
		t.Fatalf("data source type name = %q", metadataResponse.TypeName)
	}

	schemaResponse := datasource.SchemaResponse{}
	dataSource.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResponse)
	if len(schemaResponse.Schema.GetAttributes()) == 0 {
		t.Fatal("data source schema has no attributes")
	}

	invalidResponse := datasource.ConfigureResponse{}
	dataSource.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: "invalid"}, &invalidResponse)
	if !invalidResponse.Diagnostics.HasError() {
		t.Fatal("Configure() diagnostics = nil, want invalid provider data error")
	}

	validResponse := datasource.ConfigureResponse{}
	dataSource.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: &trendmicro.Client{}}, &validResponse)
	if validResponse.Diagnostics.HasError() || dataSource.client == nil {
		t.Fatalf("Configure() diagnostics = %v, client = %v", validResponse.Diagnostics, dataSource.client)
	}
}

func TestCAMCloudAccountsDataSourceRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/beta/cam/awsAccounts" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalCount":1,"count":1,"items":[{"id":"account-1","roleArn":"arn:aws:iam::123456789012:role/visionone_role","name":"test","description":"description","state":"connected","createdDateTime":"created","updatedDateTime":"updated","lastSyncedDateTime":"synced","features":[{"id":"feature-1","regions":["us-east-1"],"templateVersion":"v1"}],"connectedSecurityServices":[{"name":"workload","instanceIds":["instance-1"]}],"sources":["terraform"],"isCAMCloudASRMEnabled":true,"isCloudASRMEditable":true,"isCloudASRMEnabled":true,"isTerraformDeployed":true,"cloudAssetCount":3}]}`))
	}))
	defer server.Close()

	dataSource := &CAMCloudAccountsDataSource{client: &awsapi.CamClient{Client: &trendmicro.Client{
		HostURL:    server.URL,
		HTTPClient: server.Client(),
	}}}
	request, state := newCloudAccountsReadRequest(t, dataSource)
	response := datasource.ReadResponse{State: state}
	dataSource.Read(context.Background(), request, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("Read() diagnostics = %v", response.Diagnostics)
	}

	var model CAMAWSAccountDataSourceModel
	response.Diagnostics.Append(response.State.Get(context.Background(), &model)...)
	if len(model.CloudAccounts) != 1 || model.CloudAccounts[0].Name.ValueString() != "test" {
		t.Fatalf("Read() cloud accounts = %#v", model.CloudAccounts)
	}

	serverError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer serverError.Close()
	dataSource.client.Client.HostURL = serverError.URL
	request, state = newCloudAccountsReadRequest(t, dataSource)
	response = datasource.ReadResponse{State: state}
	dataSource.Read(context.Background(), request, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("Read() diagnostics = nil, want API error")
	}
}

func newCloudAccountsReadRequest(t *testing.T, dataSource *CAMCloudAccountsDataSource) (datasource.ReadRequest, tfsdk.State) {
	t.Helper()
	ctx := context.Background()
	schemaResponse := datasource.SchemaResponse{}
	dataSource.Schema(ctx, datasource.SchemaRequest{}, &schemaResponse)
	rawType := schemaResponse.Schema.Type().TerraformType(ctx)
	objectType, ok := rawType.(terraformtypes.Object)
	if !ok {
		t.Fatalf("data source schema type = %T, want object", rawType)
	}
	attributes := make(map[string]terraformtypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		attributes[name] = terraformtypes.NewValue(attributeType, nil)
	}
	return datasource.ReadRequest{Config: tfsdk.Config{
		Raw:    terraformtypes.NewValue(rawType, attributes),
		Schema: schemaResponse.Schema,
	}}, tfsdk.State{Schema: schemaResponse.Schema}
}

func TestNormalizePermissionModels(t *testing.T) {
	ctx := context.Background()
	actions, actionDiags := frameworktypes.ListValueFrom(ctx, frameworktypes.StringType, []string{"iam:GetRole"})
	resources, resourceDiags := frameworktypes.ListValueFrom(ctx, frameworktypes.StringType, []string{"*"})
	if actionDiags.HasError() || resourceDiags.HasError() {
		t.Fatalf("failed to create permission lists: actions=%v resources=%v", actionDiags, resourceDiags)
	}

	requirements, err := normalizePermissionModels(ctx, []deploymentPermissionModel{{
		Actions:            actions,
		Resources:          resources,
		ExecutionPrincipal: frameworktypes.StringValue("terraform_caller"),
	}})
	if err != nil || len(requirements) != 1 {
		t.Fatalf("normalizePermissionModels() = %v, %v", requirements, err)
	}

	if _, err := normalizePermissionModels(ctx, []deploymentPermissionModel{{
		Actions:            actions,
		Resources:          resources,
		ExecutionPrincipal: frameworktypes.StringValue("other"),
	}}); err == nil {
		t.Fatal("normalizePermissionModels() error = nil, want unsupported principal")
	}
	if _, err := normalizePermissionModels(ctx, nil); err == nil {
		t.Fatal("normalizePermissionModels() error = nil, want empty permission groups")
	}
}

func TestReadVerifiesTerraformAWSIdentity(t *testing.T) {
	ctx := context.Background()
	dataSource := &CAMDeploymentPreflightDataSource{
		clientFactory: func(context.Context, string) (stsAPI, iamAPI, organizationsAPI, error) {
			return mockSTSClient{identity: &sts.GetCallerIdentityOutput{
					Account: aws.String("123456789012"),
					Arn:     aws.String("arn:aws:sts::123456789012:assumed-role/TerraformRole/session"),
				}}, mockIAMClient{output: &iam.SimulatePrincipalPolicyOutput{
					EvaluationResults: []iamtypes.EvaluationResult{{
						EvalActionName: aws.String("iam:GetRole"),
						EvalDecision:   iamtypes.PolicyEvaluationDecisionTypeAllowed,
					}},
				}}, mockOrganizationsClient{}, nil
		},
	}
	request, state := newPreflightReadRequest(t, dataSource, "single", "arn:aws:iam::123456789012:role/TerraformRole")
	response := datasource.ReadResponse{State: state}
	dataSource.Read(ctx, request, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("Read() diagnostics = %v", response.Diagnostics)
	}

	request, state = newPreflightReadRequest(t, dataSource, "single", "arn:aws:iam::123456789012:role/OtherRole")
	response = datasource.ReadResponse{State: state}
	dataSource.Read(ctx, request, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("Read() diagnostics = nil, want caller identity mismatch")
	}
}

func TestReadOrganizationAndPermissionFailure(t *testing.T) {
	ctx := context.Background()
	dataSource := &CAMDeploymentPreflightDataSource{
		clientFactory: func(context.Context, string) (stsAPI, iamAPI, organizationsAPI, error) {
			return mockSTSClient{identity: &sts.GetCallerIdentityOutput{
					Account: aws.String("123456789012"),
					Arn:     aws.String("arn:aws:sts::123456789012:assumed-role/TerraformRole/session"),
				}}, mockIAMClient{output: &iam.SimulatePrincipalPolicyOutput{
					EvaluationResults: []iamtypes.EvaluationResult{{
						EvalActionName: aws.String("iam:GetRole"),
						EvalDecision:   iamtypes.PolicyEvaluationDecisionTypeAllowed,
					}},
				}}, mockOrganizationsClient{output: &organizations.DescribeOrganizationOutput{
					Organization: &organizationstypes.Organization{MasterAccountId: aws.String("123456789012")},
				}}, nil
		},
	}
	request, state := newPreflightReadRequest(t, dataSource, "organization", "arn:aws:iam::123456789012:role/TerraformRole")
	response := datasource.ReadResponse{State: state}
	dataSource.Read(ctx, request, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("organization Read() diagnostics = %v", response.Diagnostics)
	}

	dataSource.clientFactory = func(context.Context, string) (stsAPI, iamAPI, organizationsAPI, error) {
		return mockSTSClient{identity: &sts.GetCallerIdentityOutput{
				Account: aws.String("123456789012"),
				Arn:     aws.String("arn:aws:sts::123456789012:assumed-role/TerraformRole/session"),
			}}, mockIAMClient{output: &iam.SimulatePrincipalPolicyOutput{
				EvaluationResults: []iamtypes.EvaluationResult{{
					EvalActionName: aws.String("iam:GetRole"),
					EvalDecision:   iamtypes.PolicyEvaluationDecisionTypeImplicitDeny,
				}},
			}}, mockOrganizationsClient{}, nil
	}
	request, state = newPreflightReadRequest(t, dataSource, "single", "arn:aws:iam::123456789012:role/TerraformRole")
	response = datasource.ReadResponse{State: state}
	dataSource.Read(ctx, request, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("permission failure diagnostics = nil")
	}
	var failedModel CAMDeploymentPreflightDataSourceModel
	response.Diagnostics.Append(response.State.Get(ctx, &failedModel)...)
	if failedModel.Status.ValueString() != "failed" || len(failedModel.FailedPermissions.Elements()) != 1 {
		t.Fatalf("failed state = %#v", failedModel)
	}
}

func TestDefaultAWSClientFactoryAndAdditionalValidation(t *testing.T) {
	if _, _, _, err := defaultAWSClientFactory(context.Background(), "us-east-1"); err != nil {
		t.Fatalf("defaultAWSClientFactory() error = %v", err)
	}

	for _, test := range []struct {
		region    string
		partition string
	}{
		{region: "us-east-1", partition: "aws"},
		{region: "cn-north-1", partition: "aws-cn"},
		{region: "us-gov-west-1", partition: "aws-us-gov"},
		{region: "us-isob-east-1", partition: "aws-isob"},
		{region: "us-iso-east-1", partition: "aws-iso"},
		{region: "eu-isoe-west-1", partition: "aws-iso-e"},
	} {
		if err := validateRegionPartition(test.region, test.partition); err != nil {
			t.Errorf("validateRegionPartition(%q, %q) error = %v", test.region, test.partition, err)
		}
	}
	if _, _, err := checkOrganizationManagementAccount(context.Background(), mockOrganizationsClient{output: &organizations.DescribeOrganizationOutput{}}, "123456789012"); err == nil {
		t.Fatal("checkOrganizationManagementAccount() error = nil, want missing management account")
	}
	if _, err := normalizePrincipalARN("not-an-arn"); err == nil {
		t.Fatal("normalizePrincipalARN() error = nil, want malformed ARN error")
	}
	if _, err := normalizePrincipalARN("arn:aws:sts::123456789012:assumed-role"); err == nil {
		t.Fatal("normalizePrincipalARN() error = nil, want invalid assumed-role error")
	}
	if _, ok := findResourceSpecificResult(nil, "*"); ok {
		t.Fatal("findResourceSpecificResult() found a result in an empty list")
	}
}

func newPreflightReadRequest(t *testing.T, dataSource *CAMDeploymentPreflightDataSource, deploymentType, expectedCallerARN string) (datasource.ReadRequest, tfsdk.State) {
	t.Helper()
	ctx := context.Background()
	schemaResponse := datasource.SchemaResponse{}
	dataSource.Schema(ctx, datasource.SchemaRequest{}, &schemaResponse)
	rawType := schemaResponse.Schema.Type().TerraformType(ctx)
	objectType, ok := rawType.(terraformtypes.Object)
	if !ok {
		t.Fatalf("data source schema type = %T, want object", rawType)
	}

	permissionType, ok := objectType.AttributeTypes["deployment_permissions"].(terraformtypes.List)
	if !ok {
		t.Fatalf("deployment_permissions type = %T, want list", objectType.AttributeTypes["deployment_permissions"])
	}
	permissionObjectType, ok := permissionType.ElementType.(terraformtypes.Object)
	if !ok {
		t.Fatalf("permission element type = %T, want object", permissionType.ElementType)
	}
	permissionValues := map[string]terraformtypes.Value{
		"actions":             terraformtypes.NewValue(permissionObjectType.AttributeTypes["actions"], []terraformtypes.Value{terraformtypes.NewValue(terraformtypes.String, "iam:GetRole")}),
		"resources":           terraformtypes.NewValue(permissionObjectType.AttributeTypes["resources"], []terraformtypes.Value{terraformtypes.NewValue(terraformtypes.String, "*")}),
		"execution_principal": terraformtypes.NewValue(permissionObjectType.AttributeTypes["execution_principal"], "terraform_caller"),
	}
	attributes := make(map[string]terraformtypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		attributes[name] = terraformtypes.NewValue(attributeType, nil)
	}
	attributes["deployment_type"] = terraformtypes.NewValue(terraformtypes.String, deploymentType)
	attributes["aws_region"] = terraformtypes.NewValue(terraformtypes.String, "us-east-1")
	attributes["cloud_account_id"] = terraformtypes.NewValue(terraformtypes.String, "123456789012")
	attributes["expected_caller_arn"] = terraformtypes.NewValue(terraformtypes.String, expectedCallerARN)
	attributes["deployment_permissions"] = terraformtypes.NewValue(permissionType, []terraformtypes.Value{
		terraformtypes.NewValue(permissionObjectType, permissionValues),
	})

	return datasource.ReadRequest{Config: tfsdk.Config{
		Raw:    terraformtypes.NewValue(rawType, attributes),
		Schema: schemaResponse.Schema,
	}}, tfsdk.State{Schema: schemaResponse.Schema}
}

func TestExecutePreflight(t *testing.T) {
	result, err := executePreflight(
		context.Background(),
		mockSTSClient{identity: &sts.GetCallerIdentityOutput{
			Account: aws.String("123456789012"),
			Arn:     aws.String("arn:aws:sts::123456789012:assumed-role/TerraformRole/session"),
		}},
		mockIAMClient{output: &iam.SimulatePrincipalPolicyOutput{
			EvaluationResults: []iamtypes.EvaluationResult{
				{EvalActionName: aws.String("iam:GetRole"), EvalResourceName: aws.String("*"), EvalDecision: iamtypes.PolicyEvaluationDecisionTypeAllowed, MissingContextValues: []string{"aws:RequestedRegion"}},
				{EvalActionName: aws.String("iam:CreateRole"), EvalResourceName: aws.String("*"), EvalDecision: iamtypes.PolicyEvaluationDecisionTypeImplicitDeny},
			},
		}},
		"us-east-1",
		[]permissionRequirement{{actions: []string{"iam:GetRole", "iam:CreateRole"}, resources: []string{"*"}}},
	)
	if err != nil {
		t.Fatalf("executePreflight() error = %v", err)
	}
	if len(result.checked) != 2 {
		t.Fatalf("checked permissions = %d, want 2", len(result.checked))
	}
	if len(result.failed) != 1 || result.failed[0] != "iam:CreateRole on * (implicitDeny)" {
		t.Fatalf("failed permissions = %v", result.failed)
	}
	if len(result.warnings) != 1 || result.warnings[0] != "iam:GetRole on * has missing simulation context: aws:RequestedRegion" {
		t.Fatalf("warnings = %v", result.warnings)
	}
}

func TestExecutePreflightUsesResourceSpecificResults(t *testing.T) {
	result, err := executePreflight(
		context.Background(),
		mockSTSClient{identity: &sts.GetCallerIdentityOutput{
			Account: aws.String("123456789012"),
			Arn:     aws.String("arn:aws:iam::123456789012:role/TerraformRole"),
		}},
		mockIAMClient{output: &iam.SimulatePrincipalPolicyOutput{
			EvaluationResults: []iamtypes.EvaluationResult{{
				EvalActionName: aws.String("s3:GetObject"),
				EvalDecision:   iamtypes.PolicyEvaluationDecisionTypeExplicitDeny,
				OrganizationsDecisionDetail: &iamtypes.OrganizationsDecisionDetail{
					AllowedByOrganizations: false,
				},
				ResourceSpecificResults: []iamtypes.ResourceSpecificResult{
					{EvalResourceName: aws.String("arn:aws:s3:::allowed/object"), EvalResourceDecision: iamtypes.PolicyEvaluationDecisionTypeAllowed},
					{EvalResourceName: aws.String("arn:aws:s3:::denied/object"), EvalResourceDecision: iamtypes.PolicyEvaluationDecisionTypeExplicitDeny},
				},
			}},
		}},
		"us-east-1",
		[]permissionRequirement{{actions: []string{"s3:GetObject"}, resources: []string{"arn:aws:s3:::allowed/*", "arn:aws:s3:::denied/*"}}},
	)
	if err != nil {
		t.Fatalf("executePreflight() error = %v", err)
	}
	if len(result.checked) != 2 {
		t.Fatalf("checked permissions = %d, want 2", len(result.checked))
	}
	if len(result.failed) != 1 || result.failed[0] != "s3:GetObject on arn:aws:s3:::denied/* (explicitDeny, organizations_scp=denied)" {
		t.Fatalf("failed permissions = %v", result.failed)
	}
}

func TestExecutePreflightPaginatesSimulationResults(t *testing.T) {
	result, err := executePreflight(
		context.Background(),
		mockSTSClient{identity: &sts.GetCallerIdentityOutput{
			Account: aws.String("123456789012"),
			Arn:     aws.String("arn:aws:iam::123456789012:role/TerraformRole"),
		}},
		mockIAMClient{
			output: &iam.SimulatePrincipalPolicyOutput{
				IsTruncated:       true,
				Marker:            aws.String("next-page"),
				EvaluationResults: []iamtypes.EvaluationResult{{EvalActionName: aws.String("iam:GetRole"), EvalDecision: iamtypes.PolicyEvaluationDecisionTypeAllowed}},
			},
			next: &iam.SimulatePrincipalPolicyOutput{
				EvaluationResults: []iamtypes.EvaluationResult{{EvalActionName: aws.String("iam:CreateRole"), EvalDecision: iamtypes.PolicyEvaluationDecisionTypeAllowed}},
			},
		},
		"us-east-1",
		[]permissionRequirement{{actions: []string{"iam:GetRole", "iam:CreateRole"}, resources: []string{"*"}}},
	)
	if err != nil {
		t.Fatalf("executePreflight() error = %v", err)
	}
	if len(result.checked) != 2 || len(result.failed) != 0 {
		t.Fatalf("executePreflight() checked = %v, failed = %v, want two checked and no failures", result.checked, result.failed)
	}
}

func TestExecutePreflightPropagatesAWSFailures(t *testing.T) {
	_, err := executePreflight(
		context.Background(),
		mockSTSClient{err: errors.New("invalid token")},
		mockIAMClient{},
		"us-east-1",
		nil,
	)
	if err == nil {
		t.Fatal("executePreflight() error = nil, want STS error")
	}
}

func TestExecutePreflightReportsMissingSimulationPermission(t *testing.T) {
	_, err := executePreflight(
		context.Background(),
		mockSTSClient{identity: &sts.GetCallerIdentityOutput{
			Account: aws.String("123456789012"),
			Arn:     aws.String("arn:aws:iam::123456789012:role/TerraformRole"),
		}},
		mockIAMClient{err: mockAWSAPIError{code: "AccessDeniedException"}},
		"us-east-1",
		[]permissionRequirement{{actions: []string{"iam:GetRole"}, resources: []string{"*"}}},
	)
	if err == nil {
		t.Fatal("executePreflight() error = nil, want missing simulator permission error")
	}
	for _, expected := range []string{
		"lacks iam:SimulatePrincipalPolicy",
		"required to run the deployment preflight",
		"grant iam:SimulatePrincipalPolicy",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("executePreflight() error = %q, want it to contain %q", err, expected)
		}
	}
}

func TestFormatInsufficientPermissionsError(t *testing.T) {
	got := formatInsufficientPermissionsError(
		"arn:aws:sts::123456789012:assumed-role/ReadOnly/session",
		"organization",
		[]string{
			"cloudformation:CreateStackSet on * (implicitDeny, missing_context=s3express:SessionMode)",
			"cloudformation:UpdateStackSet on * (implicitDeny, missing_context=s3express:SessionMode)",
			"iam:CreateRole on * (implicitDeny, missing_context=s3express:SessionMode)",
		},
	)

	checks := []string{
		"The Terraform caller cannot deploy this CAM package.\nCaller: arn:aws:sts::123456789012:assumed-role/ReadOnly/session\nDeployment type: organization\nFailed permission checks: 3",
		"Missing or denied permissions by AWS service:",
		"  cloudformation:\n    - CreateStackSet on *\n    - UpdateStackSet on *",
		"  iam:\n    - CreateRole on *",
		"What to do:",
		"read-only credentials cannot deploy this package.",
		"terraform.tfvars under deployment_permissions",
		"For organization deployments, run Terraform with credentials from the AWS Organizations management account.",
		"AWS simulation details:\n  - implicitDeny, missing_context=s3express:SessionMode",
	}
	for _, check := range checks {
		if !strings.Contains(got, check) {
			t.Errorf("formatted error does not contain %q:\n%s", check, got)
		}
	}
}

func TestCheckOrganizationManagementAccount(t *testing.T) {
	client := mockOrganizationsClient{output: &organizations.DescribeOrganizationOutput{
		Organization: &organizationstypes.Organization{
			MasterAccountId: aws.String("111111111111"),
		},
	}}

	managementAccountID, isManagementAccount, err := checkOrganizationManagementAccount(context.Background(), client, "111111111111")
	if err != nil {
		t.Fatalf("checkOrganizationManagementAccount() error = %v", err)
	}
	if managementAccountID != "111111111111" || !isManagementAccount {
		t.Fatalf("checkOrganizationManagementAccount() = %q, %t, want management account and true", managementAccountID, isManagementAccount)
	}

	managementAccountID, isManagementAccount, err = checkOrganizationManagementAccount(context.Background(), client, "222222222222")
	if err != nil {
		t.Fatalf("checkOrganizationManagementAccount() error = %v", err)
	}
	if managementAccountID != "111111111111" || isManagementAccount {
		t.Fatalf("checkOrganizationManagementAccount() = %q, %t, want management account and false", managementAccountID, isManagementAccount)
	}
}

func TestValidateOrganizationCaller(t *testing.T) {
	if err := validateCallerAccount("111111111111", "111111111111"); err != nil {
		t.Fatalf("validateCallerAccount() error = %v", err)
	}
	if err := validateCallerAccount("111111111111", "222222222222"); err == nil {
		t.Fatal("validateCallerAccount() error = nil, want account mismatch")
	}
}

func TestValidateCallerIdentity(t *testing.T) {
	if err := validateCallerIdentity(
		"arn:aws:sts::123456789012:assumed-role/TerraformRole/session",
		"arn:aws:iam::123456789012:role/TerraformRole",
	); err != nil {
		t.Fatalf("validateCallerIdentity() error = %v", err)
	}
	if err := validateCallerIdentity(
		"arn:aws:sts::123456789012:assumed-role/OtherRole/session",
		"arn:aws:iam::123456789012:role/TerraformRole",
	); err == nil {
		t.Fatal("validateCallerIdentity() error = nil, want principal mismatch")
	}
}

func TestCheckOrganizationManagementAccountPropagatesAWSFailures(t *testing.T) {
	_, _, err := checkOrganizationManagementAccount(
		context.Background(),
		mockOrganizationsClient{err: errors.New("access denied")},
		"111111111111",
	)
	if err == nil {
		t.Fatal("checkOrganizationManagementAccount() error = nil, want Organizations error")
	}
}

func TestValidateRegionPartition(t *testing.T) {
	if err := validateRegionPartition("cn-north-1", "aws"); err == nil {
		t.Fatal("validateRegionPartition() error = nil, want partition mismatch")
	}
	if err := validateRegionPartition("us-gov-west-1", "aws-us-gov"); err != nil {
		t.Fatalf("validateRegionPartition() error = %v", err)
	}
}
