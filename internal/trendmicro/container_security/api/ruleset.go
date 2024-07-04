package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"terraform-provider-vision-one/pkg/dto"
)

func (c *CsClient) CreateRuleset(data *dto.CreateRulesetRequest) (*dto.RulesetResponse, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v3.0/containerSecurity/rulesets", c.Client.HostURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rulesetLocation := resp.Header.Get("Location")

	parsedURL, err := url.Parse(rulesetLocation)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %v", err)
	}

	// Use the path.Base function to get the last element of the path
	rulesetID := path.Base(parsedURL.Path)

	createdRuleset, err := c.GetRuleset(rulesetID)
	if err != nil {
		return nil, err
	}

	return createdRuleset, nil
}

func (c *CsClient) ListRulesets() (*dto.ListRulesetsResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v3.0/containerSecurity/rulesets", c.Client.HostURL), http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.Client.DoRequest(req)
	if err != nil {
		return nil, err
	}

	resp := dto.ListRulesetsResponse{}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *CsClient) GetRuleset(id string) (*dto.RulesetResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v3.0/containerSecurity/rulesets/%s", c.Client.HostURL, id), http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.Client.DoRequest(req)
	if err != nil {
		return nil, err
	}

	resp := dto.RulesetResponse{}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *CsClient) UpdateRuleset(id string, data *dto.CreateRulesetRequest) (*dto.RulesetResponse, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v3.0/containerSecurity/rulesets/%s", c.Client.HostURL, id), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	createdRuleset, err := c.GetRuleset(id)
	if err != nil {
		return nil, err
	}

	return createdRuleset, nil
}

func (c *CsClient) DeleteRuleset(id string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v3.0/containerSecurity/rulesets/%s", c.Client.HostURL, id), http.NoBody)
	if err != nil {
		return err
	}

	_, err = c.Client.DoRequest(req)
	if err != nil {
		return err
	}

	return nil
}
