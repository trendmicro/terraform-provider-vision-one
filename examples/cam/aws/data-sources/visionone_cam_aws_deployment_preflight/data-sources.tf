terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "<your_vision_one_api_key>"
  regional_fqdn = "<regional_fqdn>"
}

data "visionone_cam_aws_deployment_preflight" "preflight" {
  deployment_type     = "organization"
  aws_region          = "us-east-1"
  cloud_account_id    = "123456789012"
  expected_caller_arn = "arn:aws:iam::123456789012:role/TerraformRole"

  # Replace this list with the CAM-generated permissions for the deployment.
  deployment_permissions = [
    {
      actions             = ["iam:GetRole"]
      resources           = ["*"]
      execution_principal = "terraform_caller"
    },
    {
      actions             = ["iam:CreateRole"]
      resources           = ["*"]
      execution_principal = "terraform_caller"
    },
    {
      actions             = ["organizations:DescribeOrganization"]
      resources           = ["*"]
      execution_principal = "terraform_caller"
    },
  ]
}

output "preflight_status" {
  value = data.visionone_cam_aws_deployment_preflight.preflight.status
}

output "preflight_diagnostics" {
  value = {
    caller_arn            = data.visionone_cam_aws_deployment_preflight.preflight.caller_arn
    checked_permissions   = data.visionone_cam_aws_deployment_preflight.preflight.checked_permissions
    failed_permissions    = data.visionone_cam_aws_deployment_preflight.preflight.failed_permissions
    management_account_id = data.visionone_cam_aws_deployment_preflight.preflight.management_account_id
    is_management_account = data.visionone_cam_aws_deployment_preflight.preflight.is_management_account
    warnings              = data.visionone_cam_aws_deployment_preflight.preflight.warnings
  }
}
