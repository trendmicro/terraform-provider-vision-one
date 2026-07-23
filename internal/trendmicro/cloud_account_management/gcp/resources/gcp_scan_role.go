package resources

import (
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func NewGCPScanRole() resource.Resource {
	return &IAMCustomRole{
		client:       &api.CamClient{},
		typeName:     config.RESOURCE_TYPE_GCP_SCAN_ROLE,
		resourceDesc: "Trend Micro Vision One Cloud Account Management GCP read-only scan role. Creates a custom GCP IAM role that holds only read permissions (resource hierarchy discovery plus Cloud Asset Inventory read) for GCP auto-detection. Grant it once at the organization or folder node via `node_scan_roles` on `visionone_cam_service_account_integration`; projects under the node — including projects created later — inherit it through IAM. Unlike `visionone_cam_iam_custom_role`, it never includes deploy/write permissions, so enabling a feature only appends that feature's read permissions.",
		roleIDPrefix: config.GCP_SCAN_ROLE_NAME,
		core:         config.GCP_SCAN_ROLE_CORE_PERMISSIONS,
		featureTable: config.SCAN_FEATURE_PERMISSIONS,
	}
}
