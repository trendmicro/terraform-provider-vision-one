# Look up CRM account ID by AWS Account ID
data "visionone_crm_account" "aws_example" {
  aws_account_id = "123456789012"
}

# Look up CRM account ID by Azure Subscription ID
data "visionone_crm_account" "azure_example" {
  azure_subscription_id = "00000000-0000-0000-0000-000000000000"
}

# Look up CRM account ID by GCP Project ID
data "visionone_crm_account" "gcp_example" {
  gcp_project_id = "my-gcp-project"
}

# Use the CRM account ID in other CRM resources
output "crm_account_id_for_aws" {
  value = data.visionone_crm_account.aws_example.id
}

output "crm_account_id_for_azure" {
  value = data.visionone_crm_account.azure_example.id
}

output "crm_account_id_for_gcp" {
  value = data.visionone_crm_account.gcp_example.id
}