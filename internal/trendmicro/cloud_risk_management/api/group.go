package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"
)

func (c *CrmClient) CreateGroup(data *cloud_risk_management_dto.CreateGroupRequest) (*cloud_risk_management_dto.GroupResource, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	apiUrl := fmt.Sprintf("%s/beta/cloudPosture/groups", c.Client.HostURL)
	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	groupID, err := c.Client.ExtractIDFromLocationHeader(resp.Header)
	if err != nil {
		return nil, err
	}

	return &cloud_risk_management_dto.GroupResource{
		ID: groupID,
	}, nil
}

func (c *CrmClient) GetGroup(groupId string) (*cloud_risk_management_dto.GroupResource, error) {
	apiUrl := fmt.Sprintf("%s/beta/cloudPosture/groups/%s", c.Client.HostURL, groupId)

	req, err := http.NewRequest("GET", apiUrl, http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.Client.DoRequest(req)
	if err != nil {
		return nil, err
	}

	group := cloud_risk_management_dto.GroupResource{}
	err = json.Unmarshal(body, &group)
	if err != nil {
		return nil, err
	}

	return &group, nil
}

func (c *CrmClient) UpdateGroup(groupId string, data *cloud_risk_management_dto.UpdateGroupRequest) error {
	apiUrl := fmt.Sprintf("%s/beta/cloudPosture/groups/%s", c.Client.HostURL, groupId)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	_, err = c.Client.DoRequest(req)
	if err != nil {
		return err
	}

	return nil
}

func (c *CrmClient) DeleteGroup(groupId string) error {
	apiUrl := fmt.Sprintf("%s/beta/cloudPosture/groups/%s", c.Client.HostURL, groupId)
	req, err := http.NewRequest("DELETE", apiUrl, http.NoBody)
	if err != nil {
		return err
	}

	_, err = c.Client.DoRequest(req)
	if err != nil {
		return err
	}

	return nil
}
