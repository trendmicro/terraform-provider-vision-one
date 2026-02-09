package cloud_risk_management_dto

type AccountResource struct {
	ID string `json:"id"`
}

type ListAccountsResponse struct {
	Items []AccountResource `json:"items"`
}
