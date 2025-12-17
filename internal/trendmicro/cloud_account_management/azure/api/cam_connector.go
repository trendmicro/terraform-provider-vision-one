package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type SecurityService struct {
	Name        string   `json:"name"`
	InstanceIds []string `json:"instanceIds"`
}

type CreateSubscriptionRequest struct {
	ApplicationID             string            `json:"applicationId"`
	ConnectedSecurityServices []SecurityService `json:"connectedSecurityServices"`
	Description               string            `json:"description"`
	IsCAMCloudASRMEnabled     bool              `json:"isCAMCloudASRMEnabled"`
	Name                      string            `json:"name"`
	SubscriptionID            string            `json:"subscriptionId"`
	TenantID                  string            `json:"tenantId"`
}

type ModifySubscriptionRequest struct {
	ApplicationID             string            `json:"applicationId"`
	ConnectedSecurityServices []SecurityService `json:"connectedSecurityServices,omitempty"`
	Description               string            `json:"description"`
	IsCAMCloudASRMEnabled     bool              `json:"isCAMCloudASRMEnabled,omitempty"`
	Name                      string            `json:"name"`
	SubscriptionID            string            `json:"subscriptionId,omitempty"`
	TenantID                  string            `json:"tenantId"`
}

type SubscriptionResponse struct {
	ApplicationID             string            `json:"applicationId"`
	CamDeployedRegion         string            `json:"camDeployedRegion,omitempty"`
	CloudAssetCount           int               `json:"cloudAssetCount,omitempty"`
	ConnectedSecurityServices []SecurityService `json:"connectedSecurityServices,omitempty"`
	CreatedDateTime           string            `json:"createdDateTime"`
	Description               string            `json:"description,omitempty"`
	IsCAMCloudASRMEnabled     bool              `json:"isCAMCloudASRMEnabled,omitempty"`
	IsCloudASRMEditable       bool              `json:"isCloudASRMEditable,omitempty"`
	IsCloudASRMEnabled        bool              `json:"isCloudASRMEnabled,omitempty"`
	LastSyncedDateTime        string            `json:"lastSyncedDateTime,omitempty"`
	Name                      string            `json:"name"`
	Sources                   []string          `json:"sources,omitempty"`
	State                     string            `json:"state"`
	SubscriptionID            string            `json:"id"`
	TenantID                  string            `json:"tenantId"`
	UpdatedDateTime           string            `json:"updatedDateTime"`
}

func (c *CamClient) CreateSubscription(data *CreateSubscriptionRequest) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	fmt.Printf("Preparing to create subscription with data: %s\n", string(jsonData))

	// Attempt to read the subscription to determine if it already exists
	describeResp, err := c.ReadSubscription(data.SubscriptionID)
	if err != nil {
		if !strings.Contains(err.Error(), `"code": "NotFound"`) {
			return fmt.Errorf("failed to verify subscription existence: %w", err)
		}
		fmt.Printf("Subscription not found, proceeding to create new subscription: %s\n", data.SubscriptionID)
	}

	var resp *http.Response
	var postRequestErr error
	if describeResp != nil && describeResp.ApplicationID != "" {
		// If the subscription already exists, we will modify it instead of creating a new one
		fmt.Printf("Subscription already exists, modifying subscription: %s\n", data.SubscriptionID)
		url := fmt.Sprintf("%s/beta/cam/azureSubscriptions/%s", c.Client.HostURL, data.SubscriptionID)
		modifyJsonData, err := json.Marshal(data)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(modifyJsonData))
		if err != nil {
			return err
		}

		resp, err = c.Client.DoRequestWithFullResponse(req)
		if err != nil {
			return err
		}

		defer resp.Body.Close()
	} else {
		fmt.Printf("Creating new subscription: %s\n", data.SubscriptionID)
		// Retry logic for creating a new subscription
		// This will retry up to 3 times with exponential backoff
		// to handle potential transient issues
		// This is useful if the app registration creation is not yet complete
		maxRetries := 3
		baseDelay := 5 * time.Second

		for attempt := 0; attempt <= maxRetries; attempt++ {
			req, err := http.NewRequest("POST", fmt.Sprintf("%s/beta/cam/azureSubscriptions", c.Client.HostURL), bytes.NewBuffer(jsonData))
			if err != nil {
				return err
			}

			if attempt < maxRetries {
				delay := baseDelay * time.Duration(1<<attempt)
				time.Sleep(delay)
			}

			resp, postRequestErr = c.Client.DoRequestWithFullResponse(req)
			if postRequestErr == nil {
				break
			}
			fmt.Printf("Attempting to retry Azure subscription connection. Waiting for app registration propagation...\n")
			fmt.Printf("Azure subscription connection attempt %d of %d failed. Error: %v\n", attempt+1, maxRetries+1, postRequestErr)
		}

		if postRequestErr != nil {
			return fmt.Errorf("subscription connection failed after retries: %v", postRequestErr)
		}

		defer resp.Body.Close()
	}

	return nil
}

func (c *CamClient) ReadSubscription(subscriptionID string) (*SubscriptionResponse, error) {
	url := fmt.Sprintf("%s/beta/cam/azureSubscriptions/%s", c.Client.HostURL, subscriptionID)

	req, err := http.NewRequest("GET", url, http.NoBody)
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
	describeSubscriptionResponse := SubscriptionResponse{}
	err = json.Unmarshal(body, &describeSubscriptionResponse)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return &describeSubscriptionResponse, nil
}

func (c *CamClient) UpdateSubscription(subscriptionID string, data *ModifySubscriptionRequest) error {
	url := fmt.Sprintf("%s/beta/cam/azureSubscriptions/%s", c.Client.HostURL, subscriptionID)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func (c *CamClient) DeleteSubscription(subscriptionID string) error {
	url := fmt.Sprintf("%s/beta/cam/azureSubscriptions/%s", c.Client.HostURL, subscriptionID)

	req, err := http.NewRequest("DELETE", url, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return err
	}

	defer resp.Body.Close()

	return nil
}
