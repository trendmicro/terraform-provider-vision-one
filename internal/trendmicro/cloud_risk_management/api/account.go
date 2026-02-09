package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"
)

type CloudProviderFilter struct {
	AwsAccountID        string
	AzureSubscriptionID string
	GcpProjectID        string
	OciCompartmentID    string
	AlibabaAccountID    string
}

func (c *CrmClient) ListAccounts(cloudProviderFilter *CloudProviderFilter) (*cloud_risk_management_dto.ListAccountsResponse, error) {
	apiURL := fmt.Sprintf("%s/v3.0/cloudRiskManagement/accounts", c.Client.HostURL)
	req, err := http.NewRequest("GET", apiURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	// Build Filter
	var filterStr string
	switch {
	case cloudProviderFilter.AwsAccountID != "":
		filterStr = fmt.Sprintf("awsAccountId eq '%s'", cloudProviderFilter.AwsAccountID)
	case cloudProviderFilter.AzureSubscriptionID != "":
		filterStr = fmt.Sprintf("azureSubscriptionId eq '%s'", cloudProviderFilter.AzureSubscriptionID)
	case cloudProviderFilter.GcpProjectID != "":
		filterStr = fmt.Sprintf("gcpProjectId eq '%s'", cloudProviderFilter.GcpProjectID)
	case cloudProviderFilter.OciCompartmentID != "":
		filterStr = fmt.Sprintf("ociCompartmentId eq '%s'", cloudProviderFilter.OciCompartmentID)
	case cloudProviderFilter.AlibabaAccountID != "":
		filterStr = fmt.Sprintf("alibabaAccountId eq '%s'", cloudProviderFilter.AlibabaAccountID)
	default:
		return nil, fmt.Errorf("exactly one cloud provider filter must be provided (AwsAccountID, AzureSubscriptionID, GcpProjectID, OciCompartmentID, or AlibabaAccountID)")
	}
	req.Header.Set("TMV1-Filter", filterStr)

	resp, err := c.Client.DoRequest(req)
	if err != nil {
		return nil, err
	}

	accounts := cloud_risk_management_dto.ListAccountsResponse{}
	err = json.Unmarshal(resp, &accounts)
	if err != nil {
		return nil, err
	}

	return &accounts, nil
}
