package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
)

type OrganizationDetails struct {
	DisplayName      string `json:"displayName,omitempty"`
	ExcludedProjects string `json:"excludedProjects,omitempty"`
	ID               string `json:"id,omitempty"`
}

type FolderDetails struct {
	DisplayName      string `json:"displayName,omitempty"`
	ExcludedProjects string `json:"excludedProjects,omitempty"`
	ID               string `json:"id,omitempty"`
}

type CreateProjectRequest struct {
	CamDeployedRegion         string                         `json:"camDeployedRegion" validate:"omitempty,max=254"`
	ConnectedSecurityServices []cam.ConnectedSecurityService `json:"connectedSecurityServices"`
	Description               string                         `json:"description" validate:"omitempty,max=254"`
	Folder                    *FolderDetails                 `json:"folder" validate:"omitempty"`
	IsCAMCloudASRMEnabled     bool                           `json:"isCAMCloudASRMEnabled" validate:"omitempty"`
	IsTFProviderDeployed      bool                           `json:"isTFProviderDeployed" validate:"omitempty"`
	Name                      string                         `json:"name" validate:"max=254"`
	Organization              *OrganizationDetails           `json:"organization" validate:"omitempty"`
	ProjectNumber             string                         `json:"projectNumber" validate:"omitempty,max=254"`
	ServiceAccountId          string                         `json:"serviceAccountId" validate:"omitempty,max=254"`
	ServiceAccountKey         string                         `json:"serviceAccountKey,omitempty"`
}

type ModifyProjectRequest struct {
	CamDeployedRegion         string                         `json:"camDeployedRegion" validate:"omitempty,max=254"`
	ConnectedSecurityServices []cam.ConnectedSecurityService `json:"connectedSecurityServices" validate:"omitempty"`
	Description               string                         `json:"description" validate:"omitempty,max=254"`
	Folder                    *FolderDetails                 `json:"folder" validate:"omitempty"`
	IsCAMCloudASRMEnabled     bool                           `json:"isCAMCloudASRMEnabled" validate:"omitempty"`
	IsTFProviderDeployed      bool                           `json:"isTFProviderDeployed" validate:"omitempty"`
	Name                      string                         `json:"name" validate:"max=254"`
	Organization              *OrganizationDetails           `json:"organization" validate:"omitempty"`
	ProjectNumber             string                         `json:"projectNumber" validate:"omitempty,max=254"`
	ServiceAccountId          string                         `json:"serviceAccountId" validate:"omitempty,max=254"`
	ServiceAccountKey         string                         `json:"serviceAccountKey,omitempty"`
}

type ProjectResponse struct {
	CamDeployedRegion         string                         `json:"camDeployedRegion,omitempty"`
	CloudAssetCount           int                            `json:"cloudAssetCount,omitempty"`
	ConnectedSecurityServices []cam.ConnectedSecurityService `json:"connectedSecurityServices,omitempty"`
	CreatedTime               string                         `json:"createdDateTime,omitempty"`
	Description               string                         `json:"description,omitempty"`
	IsCAMCloudASRMEnabled     bool                           `json:"isCAMCloudASRMEnabled,omitempty"`
	IsCloudASRMEditable       *bool                          `json:"isCloudASRMEditable,omitempty"`
	IsCloudASRMEnabled        *bool                          `json:"isCloudASRMEnabled,omitempty"`
	LastSyncedDateTime        string                         `json:"lastSyncedDateTime,omitempty"`
	Name                      string                         `json:"name,omitempty"`
	ProjectID                 string                         `json:"projectId,omitempty"`
	ProjectName               string                         `json:"projectName,omitempty"`
	ProjectNumber             string                         `json:"id,omitempty"`
	ServiceAccountEmail       string                         `json:"serviceAccountEmail,omitempty"` // GCP Service Account Email
	ServiceAccountID          string                         `json:"serviceAccountId,omitempty"`    // GCP Service Account Unique ID
	State                     string                         `json:"state,omitempty"`
	Sources                   []string                       `json:"sources,omitempty"`
	UpdatedDateTime           string                         `json:"updatedDateTime,omitempty"`
	Folder                    *FolderDetails                 `json:"folder,omitempty"`
	Organization              *OrganizationDetails           `json:"organization,omitempty"`
}

func (c *CamClient) CreateProject(data *CreateProjectRequest) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	fmt.Printf("Preparing to create project with data: %s\n", string(jsonData))

	// Attempt to read the project to determine if it already exists
	describeResp, err := c.ReadProject(data.ProjectNumber)
	if err != nil {
		if !strings.Contains(err.Error(), `"code": "NotFound"`) {
			return fmt.Errorf("failed to verify project existence: %w", err)
		}
		fmt.Printf("Project not found, proceeding to create new project: %s\n", data.ProjectNumber)
	}
	var resp *http.Response
	var postRequestErr error
	// Handle different scenarios based on project existence and sources
	// - If project exists with no sources (common connector): modify it
	// - If project exists with sources (Bridge/Legacy account): modify it to add TF provider
	// - If project doesn't exist: create new project
	if describeResp != nil && describeResp.ServiceAccountID != "" {
		// Project already exists (either common connector or Bridge/Legacy account)
		// We modify it to update/add TF provider deployment
		if len(describeResp.Sources) > 0 {
			fmt.Printf("Project already connected via sources %v, modifying to add TF provider: %s\n", describeResp.Sources, data.ProjectNumber)
		} else {
			fmt.Printf("Project already exists (common connector), modifying Project: %s\n", data.ProjectNumber)
		}
		url := fmt.Sprintf("%s/beta/cam/gcpProjects/%s", c.Client.HostURL, data.ProjectNumber)
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
		fmt.Printf("Creating new project: %s\n", data.ProjectNumber)
		// Retry logic for creating a new project in case of transient errors, such as the project not being fully provisioned yet
		// This will retry up to 3 times with exponential backoff
		// to handle potential transient issues
		// This is useful if the app registration creation is not yet complete
		maxRetries := 3
		baseDelay := 5 * time.Second

		for attempt := 0; attempt <= maxRetries; attempt++ {
			req, err := http.NewRequest("POST", fmt.Sprintf("%s/beta/cam/gcpProjects", c.Client.HostURL), bytes.NewBuffer(jsonData))
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
			fmt.Printf("Attempting to retry GCP project connection. Waiting for app registration propagation...\n")
			fmt.Printf("GCP project connection attempt %d of %d failed. Error: %v\n", attempt+1, maxRetries+1, postRequestErr)
		}

		if postRequestErr != nil {
			return fmt.Errorf("GCP project connection failed after retries: %v", postRequestErr)
		}

		defer resp.Body.Close()
	}
	return nil
}

func (c *CamClient) ReadProject(projectNumber string) (*ProjectResponse, error) {
	url := fmt.Sprintf("%s/beta/cam/gcpProjects/%s", c.Client.HostURL, projectNumber)

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
	describeProjectResponse := ProjectResponse{}
	err = json.Unmarshal(body, &describeProjectResponse)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return &describeProjectResponse, nil
}

func (c *CamClient) UpdateProject(projectNumber string, data *ModifyProjectRequest) error {
	url := fmt.Sprintf("%s/beta/cam/gcpProjects/%s", c.Client.HostURL, projectNumber)
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

func (c *CamClient) DeleteProject(projectNumber string) error {
	url := fmt.Sprintf("%s/beta/cam/gcpProjects/%s", c.Client.HostURL, projectNumber)

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
