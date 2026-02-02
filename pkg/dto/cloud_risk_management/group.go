package cloud_risk_management_dto

type CreateGroupRequest struct {
	Name string   `json:"name"`
	Tags []string `json:"tags,omitempty"`
}

type UpdateGroupRequest struct {
	Name string   `json:"name,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

type GroupAccount struct {
	ID string `json:"id"`
}

type GroupResource struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Tags            []string       `json:"tags"`
	Accounts        []GroupAccount `json:"accounts"`
	CreatedDateTime string         `json:"createdDateTime"`
	UpdatedDateTime string         `json:"updatedDateTime"`
}
