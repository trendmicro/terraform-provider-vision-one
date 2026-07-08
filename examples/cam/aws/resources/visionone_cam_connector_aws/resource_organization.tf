# AWS CAM connector for an AWS Organization
#
# ===== WHEN TO USE THIS EXAMPLE =====
# Use this when connecting an AWS Organization management account.
#
# ===== PREREQUISITES =====
# 1. You must be the AWS Organization management account
# 2. organization_id is the bare AWS Organization ID (o-<alphanum10-32>), obtainable via
#    data.aws_organizations_organization.current.id; OU IDs (ou-) and root IDs (r-) are also accepted
# 3. is_aws_org_mgmt_account and organization_excluded_accounts require organization_id

terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

resource "visionone_cam_connector_aws" "cam_connector_aws_org" {
  cloud_account_id        = "123456789012"
  role_arn                = "arn:aws:iam::123456789012:role/VisionOneRole"
  name                    = "CAM Connector - AWS Organization Management Account"
  description             = "CAM connector for AWS Organization management account"
  is_crem_enabled         = true
  is_aws_org_mgmt_account = true

  # organization_id: bare AWS Organization ID (e.g. data.aws_organizations_organization.current.id)
  # - Org ID format:  o-<alphanum10-32>  (e.g., o-aa111bb2cc)
  # - OU ID format:   ou-<id>-<alphanum8-32>
  # - Root ID format: r-<alphanum4-32>
  organization_id = "o-aa111bb2cc"

  # Optionally exclude specific member accounts from organization onboarding
  organization_excluded_accounts = ["999999999999"]
}
