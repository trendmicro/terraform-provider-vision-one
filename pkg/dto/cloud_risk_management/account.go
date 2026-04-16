package cloud_risk_management_dto

type AccountResource struct {
	ID string `json:"id"`
}

type ListAccountsResponse struct {
	Items []AccountResource `json:"items"`
}

type Account struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Provider            string `json:"provider"`
	AwsAccountID        string `json:"awsAccountId"`
	AzureSubscriptionID string `json:"azureSubscriptionId"`
	GcpProjectID        string `json:"gcpProjectId"`
	OciCompartmentID    string `json:"ociCompartmentId"`
	AlibabaAccountID    string `json:"alibabaAccountId"`
}
