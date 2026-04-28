# Migrate a GCP project from legacy Terraform Package Solution to Terraform Provider Solution.
# Step 2 of 3: update the CAM database record to use the new service account key.
resource "visionone_cam_gcp_project_migration" "example" {
  project_number          = "123456789012"
  name                    = "My GCP Connector"
  new_service_account_id  = visionone_cam_service_account_integration.new_sa.service_account_unique_id
  new_service_account_key = visionone_cam_service_account_integration.new_sa.private_key
}
