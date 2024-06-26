package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"terraform-provider-visionone/pkg/dto"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (c *CsClient) CreateCluster(data *dto.CreateClusterRequest) (*dto.CreateClusterResponse, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v3.0/containerSecurity/kubernetesClusters", c.Client.HostURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	createClusterResponse := dto.CreateClusterResponse{}
	err = json.Unmarshal(body, &createClusterResponse)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	clusterLocation := resp.Header.Get("Location")

	parsedURL, err := url.Parse(clusterLocation)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %v", err)
	}

	// Use the path.Base function to get the last element of the path
	clusterID := path.Base(parsedURL.Path)

	createClusterResponse.ID = clusterID

	return &createClusterResponse, nil
}

func (c *CsClient) GetClusterList() (*dto.ListClusterResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v3.0/containerSecurity/kubernetesClusters", c.Client.HostURL), http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.Client.DoRequest(req)
	if err != nil {
		return nil, err
	}

	resp := dto.ListClusterResponse{}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *CsClient) GetCluster(data *dto.GetClusterRequest) (resp *dto.GetClusterResponse, err error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v3.0/containerSecurity/kubernetesClusters/%s", c.Client.HostURL, data.ID), http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.Client.DoRequest(req)
	if err != nil {
		return resp, err
	}

	resp = &dto.GetClusterResponse{}
	err = json.Unmarshal(body, &resp.Item)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *CsClient) UpdateCurrentState(resource *dto.ClusterResourceModel) error {
	latest, err := c.GetCluster(&dto.GetClusterRequest{ID: resource.ID.ValueString()})
	if err != nil {
		return err
	}

	resource.Name = types.StringValue(latest.Item.Name)
	if latest.Item.Description != "" {
		resource.Description = types.StringValue(latest.Item.Description)
	}
	resource.Orchestrator = types.StringValue(latest.Item.Orchestrator)
	if latest.Item.PolicyId != "" {
		resource.PolicyId = types.StringValue(latest.Item.PolicyId)
	}
	if latest.Item.ResourceId != "" {
		resource.ResourceId = types.StringValue(latest.Item.ResourceId)
	}
	resource.CreatedDateTime = types.StringValue(latest.Item.CreatedDateTime)
	resource.UpdatedDateTime = types.StringValue(latest.Item.UpdatedDateTime)
	resource.LastEvaluatedDateTime = types.StringValue(latest.Item.LastEvaluatedDateTime)

	return nil
}

func (c *CsClient) UpdateCluster(clusterId string, data *dto.UpdateClusterRequest) (err error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v3.0/containerSecurity/kubernetesClusters/%s", c.Client.HostURL, clusterId), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	_, err = c.Client.DoRequest(req)
	if err != nil {
		return err
	}

	return nil
}

func (c *CsClient) DeleteCluster(data *dto.DeleteClusterRequest) (err error) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v3.0/containerSecurity/kubernetesClusters/%s", c.Client.HostURL, data.ID), http.NoBody)
	if err != nil {
		return err
	}

	_, err = c.Client.DoRequest(req)
	if err != nil {
		return err
	}

	return nil
}
